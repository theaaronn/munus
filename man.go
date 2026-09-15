package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type Todo struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

const (
	modeLists = iota
	modeTodos
	modeFullDetail
	modeTitleInput
	modeBodyInput
	modeListNameInput
)

const (
	paneLists = iota
	paneTodos
)

type model struct {
	dirs          []string // <todoListName>_munus
	dir           string   // active list dir
	file          string   // json file backing the active list
	repo          string
	level         int8
	pane          int8 // paneLists or paneTodos
	mode          int8
	todos         []Todo
	cursor        int    // cursor within active pane
	listCursor    int    // cursor within dirs
	editIdx       int    // todo index being edited, -1 for new
	titleInput    string // pending todo title
	input         strings.Builder
	status        string // last error/info message
	width, height int
	scroll        int // fullscreen detail scroll offset
}

func (m model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	return v
}

func (m model) Init() tea.Cmd {
	return nil
}

func initialModel() (model, error) {
	dirs, err := loadDirs(".")
	if err != nil {
		return model{}, fmt.Errorf("err while loading dirs from . : %v", err)
	}
	m := model{
		dirs:    dirs,
		repo:    ".",
		pane:    paneLists,
		mode:    modeLists,
		editIdx: -1,
	}
	if len(dirs) > 0 {
		if err := m.loadList(0); err != nil {
			return model{}, fmt.Errorf("from initialModel: %v", err)
		}
	}
	return m, nil
}

// loadList loads the todos of dirs[i] without changing pane focus
func (m *model) loadList(i int) error {
	m.listCursor = i
	m.dir = m.dirs[i]
	m.cursor = 0
	todos, file, err := loadTodos(m.dir)
	if err != nil {
		return err
	}
	m.todos = todos
	m.file = file
	return nil
}

func (m *model) selectList(i int) error {
	if err := m.loadList(i); err != nil {
		return err
	}
	m.pane = paneTodos
	return nil
}

func loadTodos(dir string) ([]Todo, string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, "", fmt.Errorf("at reading todos directory %v: %v", dir, err)
	}
	todos := make([]Todo, 0)
	file := ""
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			if file == "" {
				file = entry.Name()
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				return nil, "", fmt.Errorf("at open file %v: %v", entry, err)
			}
			var fileTodos []Todo
			err = json.Unmarshal(data, &fileTodos)
			if err != nil {
				return nil, "", fmt.Errorf("at unmarshalling file %v: %v", entry, err)
			}
			todos = append(todos, fileTodos...)
		}
	}
	return todos, file, nil
}

func loadDirs(rootDir string) ([]string, error) {
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_munus") && entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	return dirs, nil
}

// saveTodos writes the active list's todos back to its json file
func (m model) saveTodos() error {
	if m.dir == "" {
		return fmt.Errorf("no list selected")
	}
	file := m.file
	if file == "" {
		file = strings.TrimSuffix(m.dir, "_munus") + ".json"
	}
	data, err := json.MarshalIndent(m.todos, "", "  ")
	if err != nil {
		return fmt.Errorf("at marshalling todos: %v", err)
	}
	if err := os.WriteFile(filepath.Join(m.dir, file), data, 0o644); err != nil {
		return fmt.Errorf("at writing file %v: %v", file, err)
	}
	return nil
}

// createList makes a new <name>_munus dir and selects it
func (m *model) createList(name string) error {
	if name == "" {
		return fmt.Errorf("list name cannot be empty")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("list name cannot contain slashes")
	}
	dir := name + "_munus"
	if err := os.Mkdir(dir, 0o755); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("list %v already exists", name)
		}
		return fmt.Errorf("at creating list dir %v: %v", dir, err)
	}
	m.dirs = append(m.dirs, dir)
	if err := m.selectList(len(m.dirs) - 1); err != nil {
		return err
	}
	m.pane = paneLists
	return nil
}

func (m *model) submitTodo(title, body string) error {
	if title == "" {
		return fmt.Errorf("title cannot be empty")
	}
	if m.editIdx >= 0 {
		m.todos[m.editIdx] = Todo{Title: title, Body: body}
	} else {
		m.todos = append(m.todos, Todo{Title: title, Body: body})
		m.cursor = len(m.todos) - 1
	}
	if err := m.saveTodos(); err != nil {
		return err
	}
	m.editIdx = -1
	m.mode = modeTodos
	return nil
}

func (m *model) deleteTodo() error {
	if len(m.todos) == 0 {
		return fmt.Errorf("no todos to delete")
	}
	m.todos = append(m.todos[:m.cursor], m.todos[m.cursor+1:]...)
	if m.cursor >= len(m.todos) {
		m.cursor = max(0, len(m.todos)-1)
	}
	return m.saveTodos()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch m.mode {
		case modeLists, modeTodos:
			return m.handleNavKeys(msg)
		case modeFullDetail:
			return m.handleFullDetailKeys(msg)
		case modeTitleInput, modeBodyInput, modeListNameInput:
			return m.handleInputKeys(msg)
		}
	}
	return m, nil
}

func (m model) handleFullDetailKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "h", "left":
		m.mode = modeTodos
		m.pane = paneTodos
		m.scroll = 0
	case "up", "k":
		if m.scroll > 0 {
			m.scroll--
		}
	case "down", "j":
		scrollMax := m.fullDetailMaxScroll()
		if m.scroll < scrollMax {
			m.scroll++
		}
	case "e":
		if len(m.todos) == 0 {
			return m, nil
		}
		if m.cursor >= len(m.todos) {
			m.cursor = len(m.todos) - 1
		}
		todo := m.todos[m.cursor]
		m.editIdx = m.cursor
		m.titleInput = todo.Title
		m.input.Reset()
		m.input.WriteString(todo.Body)
		m.mode = modeBodyInput
		m.status = ""
	case "d":
		if err := m.deleteTodo(); err != nil {
			m.status = err.Error()
		} else {
			m.status = "todo deleted"
			if len(m.todos) == 0 {
				m.mode = modeTodos
			}
		}
	}
	return m, nil
}

func (m model) handleNavKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		if m.pane == paneLists {
			m.pane = paneTodos
		} else {
			m.pane = paneLists
		}
	case "right":
		if m.pane == paneLists && len(m.dirs) > 0 {
			if err := m.selectList(m.listCursor); err != nil {
				m.status = err.Error()
			}
		} else if m.pane == paneTodos && len(m.todos) > 0 {
			m.mode = modeFullDetail
		}
	case "left":
		if m.pane == paneTodos {
			m.pane = paneLists
		}
	case "up", "k":
		if m.pane == paneLists {
			if m.listCursor > 0 {
				m.listCursor--
				if err := m.loadList(m.listCursor); err != nil {
					m.status = err.Error()
				}
			}
		} else {
			m.cursor = max(0, m.cursor-1)
		}
	case "down", "j":
		if m.pane == paneLists {
			if m.listCursor < len(m.dirs)-1 {
				m.listCursor++
				if err := m.loadList(m.listCursor); err != nil {
					m.status = err.Error()
				}
			}
		} else {
			m.cursor = min(len(m.todos)-1, m.cursor+1)
		}
	case "enter":
		if m.pane == paneLists && len(m.dirs) > 0 {
			if err := m.selectList(m.listCursor); err != nil {
				m.status = err.Error()
			}
		}
		if m.pane == paneTodos && len(m.todos) > 0 {
			m.mode = modeFullDetail
		}
	case "a":
		if len(m.dirs) == 0 {
			m.status = "create a list first (n)"
			return m, nil
		}
		m.editIdx = -1
		m.input.Reset()
		m.mode = modeTitleInput
		m.status = ""
	case "e":
		if m.pane != paneTodos || len(m.todos) == 0 {
			return m, nil
		}
		m.editIdx = m.cursor
		m.input.Reset()
		m.mode = modeTitleInput
		m.status = ""
	case "d":
		if m.pane == paneTodos && len(m.todos) > 0 {
			if err := m.deleteTodo(); err != nil {
				m.status = err.Error()
			} else {
				m.status = "todo deleted"
			}
		}
	case "n":
		m.input.Reset()
		m.mode = modeListNameInput
		m.status = ""
	}
	return m, nil
}

func (m model) handleInputKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeTodos
		m.editIdx = -1
		m.input.Reset()
	case "enter":
		text := m.input.String()
		switch m.mode {
		case modeListNameInput:
			if err := m.createList(text); err != nil {
				m.status = err.Error()
				return m, nil
			}
			m.input.Reset()
			m.mode = modeTodos
			m.status = "list created"
			m.pane = paneTodos
		case modeTitleInput:
			if text == "" {
				m.status = "title cannot be empty"
				return m, nil
			}
			m.input.Reset()
			m.titleInput = text
			m.mode = modeBodyInput
		case modeBodyInput:
			m.input.Reset()
			if err := m.submitTodo(m.titleInput, text); err != nil {
				m.status = err.Error()
				return m, nil
			}
			m.mode = modeTodos
			m.status = ""
		}
	case "backspace":
		s := m.input.String()
		if runes := []rune(s); len(runes) > 0 {
			m.input.Reset()
			m.input.WriteString(string(runes[:len(runes)-1]))
		}
	case "ctrl+c":
		return m, tea.Quit
	default:
		m.input.WriteString(msg.Text)
	}
	return m, nil
}

func (m model) render() string {
	var b strings.Builder

	title := "munus"
	if m.dir != "" {
		fmt.Fprintf(&b, "%s — %s\n", title, m.dirName())
	} else {
		fmt.Fprintf(&b, "%s\n", title)
	}
	b.WriteString("\n")

	if m.mode == modeFullDetail {
		b.WriteString(m.renderFullDetail())
	} else {
		b.WriteString(m.renderColumns())
	}

	footer := m.footerText()

	footerLines := strings.Count(footer, "\n") + 1
	used := strings.Count(b.String(), "\n")
	height := m.height
	if height <= 0 {
		height = 24
	}
	if pad := height - used - footerLines; pad > 0 {
		b.WriteString(strings.Repeat("\n", pad))
	}
	b.WriteString(footer)
	return b.String()
}

// footerText builds the bottom bar prompt, status or controls
func (m model) footerText() string {
	var f strings.Builder
	switch m.mode {
	case modeTitleInput:
		if m.editIdx >= 0 {
			fmt.Fprintf(&f, "edit title: %s█", m.input.String())
		} else {
			fmt.Fprintf(&f, "new todo — title: %s█", m.input.String())
		}
	case modeBodyInput:
		fmt.Fprintf(&f, "description: %s█", m.input.String())
	case modeListNameInput:
		fmt.Fprintf(&f, "new list name (creates <name>_munus): %s█", m.input.String())
	default:
		if m.status != "" {
			fmt.Fprintf(&f, "%s\n", m.status)
		}
		f.WriteString(m.helpLine())
	}
	return f.String()
}

func (m model) helpLine() string {
	switch m.mode {
	case modeFullDetail:
		return "esc back • e edit • d delete • j/k browse • q quit"
	default:
		return "←/→ panes • enter open • a add • e edit • d delete • n new list • j/k move • q quit"
	}
}

// fullDetailLines renders the selected todo wrapped to terminal width,
// with title first then description lines
func (m model) fullDetailLines(width, viewport int) []string {
	if len(m.todos) == 0 {
		return []string{"no todos"}
	}
	if m.cursor >= len(m.todos) {
		m.cursor = len(m.todos) - 1
	}
	todo := m.todos[m.cursor]

	var lines []string
	hdr := fmt.Sprintf("title: %s", todo.Title)
	if width > 7 {
		hdr = "title: " + wrapText(todo.Title, width-7)[0]
	}
	lines = append(lines, hdr)
	if todo.Body != "" {
		lines = append(lines, "")
		lines = append(lines, wrapText(todo.Body, width)...)
	} else {
		lines = append(lines, "", "(no description)")
	}
	if len(lines) > viewport && viewport > 0 && m.scroll > 0 {
		end := min(len(lines), m.scroll+viewport)
		lines = lines[m.scroll:end]
	}
	return lines
}

func (m model) fullDetailMaxScroll() int {
	viewport := m.viewport()
	if viewport <= 0 {
		return 0
	}
	full := m.fullDetailLines(m.width, 100000)
	return max(0, len(full)-viewport)
}

func (m model) viewport() int {
	// header(2) + footer(1)
	return m.height - 3
}

// renderFullDetail shows the selected todo across the entire width
func (m model) renderFullDetail() string {
	total := m.width
	if total <= 0 {
		total = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}
	viewport := max(1, height-3)

	var s strings.Builder
	for _, line := range m.fullDetailLines(total, viewport) {
		s.WriteString(pad(line, total) + "\n")
	}
	return s.String()
}

// renderColumns lays out content side by side in a yazi-like view:
// normal mode:  | lists | todos  |
// detail mode:  | todos | detail |
func (m model) renderColumns() string {
	gap := 2
	total := m.width
	if total <= 0 {
		total = 80
	}
	leftW := max(16, total/3)
	rightW := max(10, total-leftW-gap)

	var leftLines, rightLines []string
	leftHdr, rightHdr := "", ""
	if m.pane == paneTodos && m.dir != "" {
		leftHdr, rightHdr = "todos", "detail"
		leftLines = m.todoLines(true)
		rightLines = m.detailLines()
	} else {
		leftHdr, rightHdr = "lists", "todos"
		leftLines = m.listLines()
		rightLines = m.todoLines(false)
	}

	leftLines = boxLines(append([]string{leftHdr}, leftLines...), leftW)
	rightLines = boxLines(append([]string{rightHdr}, rightLines...), rightW)

	var s strings.Builder
	n := max(len(leftLines), len(rightLines))
	for i := 0; i < n; i++ {
		l, r := "", ""
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}
		fmt.Fprintf(&s, "%s%s\n", pad(l, leftW), strings.Repeat(" ", gap)+r)
	}
	return s.String()
}

// listLines returns one line per list
func (m model) listLines() []string {
	if len(m.dirs) == 0 {
		return []string{"(no lists — press n)"}
	}
	lines := make([]string, len(m.dirs))
	for idx, dir := range m.dirs {
		marker := "  "
		if m.pane == paneLists && m.listCursor == idx {
			marker = "> "
		} else if m.dir == dir {
			marker = "* "
		}
		lines[idx] = marker + strings.TrimSuffix(dir, "_munus")
	}
	return lines
}

// todoLines returns one line per todo; foldable in column 2
func (m model) todoLines(showTitles bool) []string {
	if m.dir == "" {
		return []string{"(no list selected)"}
	}
	if len(m.todos) == 0 {
		return []string{"(empty — press 'a' to add a todo)"}
	}
	lines := make([]string, len(m.todos))
	for idx, todo := range m.todos {
		if showTitles {
			marker := "  "
			if m.cursor == idx {
				marker = "> "
			}
			preview := todo.Body
			lines[idx] = marker + todo.Title
			if preview != "" {
				lines[idx] += "  — " + preview
			}
		} else {
			marker := "  "
			if m.pane == paneTodos && m.cursor == idx {
				marker = "> "
			}
			lines[idx] = marker + todo.Title
		}
	}
	return lines
}

// detailLines renders the currently selected todo in the detail column
func (m model) detailLines() []string {
	if len(m.todos) == 0 {
		return []string{"(no todos)"}
	}
	if m.cursor >= len(m.todos) {
		m.cursor = len(m.todos) - 1
	}
	todo := m.todos[m.cursor]
	lines := []string{fmt.Sprintf("title: %s", todo.Title)}
	if todo.Body != "" {
		lines = append(lines, "", todo.Body)
	} else {
		lines = append(lines, "", "(no description)")
	}
	return lines
}

// boxLines pads/truncates each line to width (first line is the header)
func boxLines(lines []string, width int) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = pad(truncate(line, width), width)
	}
	return out
}

func pad(s string, w int) string {
	l := len([]rune(s))
	if l >= w {
		return s
	}
	return s + strings.Repeat(" ", w-l)
}

func truncate(s string, w int) string {
	rs := []rune(s)
	if len(rs) <= w {
		return s
	}
	if w <= 1 {
		return string(rs[:1])
	}
	return string(rs[:w-1]) + "…"
}

// wrapText wraps each source line at width, breaking on spaces
func wrapText(s string, w int) []string {
	if w <= 0 {
		w = 1
	}
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			out = append(out, "")
			continue
		}
		words := strings.Split(line, " ")
		cur := ""
		for _, word := range words {
			for len([]rune(word)) > w {
				rs := []rune(word)
				out = append(out, string(rs[:w]))
				word = string(rs[w:])
			}
			if cur == "" {
				cur = word
			} else if len([]rune(cur))+1+len([]rune(word)) <= w {
				cur += " " + word
			} else {
				out = append(out, cur)
				cur = word
			}
		}
		if cur != "" {
			out = append(out, cur)
		}
	}
	return out
}

func (m model) dirName() string {
	if m.dir == "" {
		return ""
	}
	return strings.TrimSuffix(m.dir, "_munus")
}

func main() {
	m, err := initialModel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "at main initialModel: %v", err)
		os.Exit(1)
	}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
