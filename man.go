package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	input         string
	status        string // last error/info message
	pendingEdit   bool   // "e" prefix pressed, waiting for t/d
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

// beginEdit routes the e-t / e-d chords to title or description edit
func (m model) beginEdit(shortcut string) {
	if len(m.todos) == 0 {
		return
	}
	if m.cursor >= len(m.todos) {
		m.cursor = len(m.todos) - 1
	}
	todo := m.todos[m.cursor]
	m.editIdx = m.cursor
	m.titleInput = todo.Title
	m.input = ""
	m.status = ""
	switch shortcut {
	case "t":
		m.input = todo.Title
		m.mode = modeTitleInput
	case "d":
		m.input = todo.Body
		m.mode = modeBodyInput
	}
}

func (m model) handleFullDetailKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.pendingEdit {
		m.pendingEdit = false
		switch msg.String() {
		case "t", "d":
			m.beginEdit(msg.String())
		case "esc":
			m.status = "edit cancelled"
		default:
			m.status = "edit: unknown command (e-t title, e-d description, esc cancel)"
		}
		return m, nil
	}
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
		m.pendingEdit = true
		m.status = "edit: t = title, d = description"
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
	if m.pendingEdit {
		m.pendingEdit = false
		switch msg.String() {
		case "t", "d":
			m.beginEdit(msg.String())
		case "esc":
			m.status = "edit cancelled"
		default:
			m.status = "edit: unknown command (e-t title, e-d description, esc cancel)"
		}
		return m, nil
	}
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
		m.input = ""
		m.mode = modeTitleInput
		m.status = ""
	case "e":
		m.pendingEdit = true
		m.status = "edit: t = title, d = description"
		return m, nil
	case "d":
		if m.pane == paneTodos && len(m.todos) > 0 {
			if err := m.deleteTodo(); err != nil {
				m.status = err.Error()
			} else {
				m.status = "todo deleted"
			}
		}
	case "n":
		m.input = ""
		m.mode = modeListNameInput
		m.status = ""
	}
	return m, nil
}

func (m model) handleInputKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "Q":
		return m, tea.Quit
	case "esc":
		m.mode = modeTodos
		m.editIdx = -1
		m.input = ""
	case "enter":
		text := m.input
		switch m.mode {
		case modeListNameInput:
			if err := m.createList(text); err != nil {
				m.status = err.Error()
				return m, nil
			}
			m.input = ""
			m.mode = modeTodos
			m.status = "list created"
			m.pane = paneTodos
		case modeTitleInput:
			if text == "" {
				m.status = "title cannot be empty"
				return m, nil
			}
			m.input = ""
			m.titleInput = text
			m.mode = modeBodyInput
		case modeBodyInput:
			m.input = ""
			if err := m.submitTodo(m.titleInput, text); err != nil {
				m.status = err.Error()
				return m, nil
			}
			m.mode = modeTodos
			m.status = ""
		}
	case "backspace":
		if runes := []rune(m.input); len(runes) > 0 {
			m.input = string(runes[:len(runes)-1])
		}
	case "ctrl+c":
		return m, tea.Quit
	default:
		m.input += msg.Text
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
			fmt.Fprintf(&f, "edit title: %s█", m.input)
		} else {
			fmt.Fprintf(&f, "new todo — title: %s█", m.input)
		}
	case modeBodyInput:
		fmt.Fprintf(&f, "description: %s█", m.input)
	case modeListNameInput:
		fmt.Fprintf(&f, "new list name (creates <name>_munus): %s█", m.input)
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
		return "←/esc back • e-t title • e-d desc • d delete • j/k browse • q quit"
	default:
		return "←/→ panes • enter open • a add • e-t title • e-d desc • d delete • n new list • j/k move • q quit"
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
	// emphasized title: bold + reverse video + underline bar, reads larger
	// than the body in the detail view
	title := wrapText(todo.Title, width)
	lines = append(lines, styleTitle(title[0]))
	lines = append(lines, titleBar(len([]rune(stripAnsi(title[0])))))
	if len(title) > 1 {
		lines = append(lines, "")
		lines = append(lines, title[1:]...)
	}
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
		text := strings.TrimSuffix(dir, "_munus")
		if m.pane == paneLists && m.listCursor == idx {
			lines[idx] = "  " + hoverStyle.Render(text)
		} else if m.dir == dir {
			lines[idx] = "* " + text
		} else {
			lines[idx] = "  " + text
		}
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
			preview := todo.Body
			line := todo.Title
			if preview != "" {
				line += "  — " + preview
			}
			if m.cursor == idx {
				lines[idx] = "  " + hoverStyle.Render(line)
			} else {
				lines[idx] = "  " + line
			}
		} else {
			if m.pane == paneTodos && m.cursor == idx {
				lines[idx] = "  " + hoverStyle.Render(todo.Title)
			} else {
				lines[idx] = "  " + todo.Title
			}
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
	return m.fullDetailLines(1<<30, 1<<30)
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
	l := visualWidth(s)
	if l >= w {
		return s
	}
	return s + strings.Repeat(" ", w-l)
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsi(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func visualWidth(s string) int {
	return len([]rune(stripAnsi(s)))
}

// truncate shortens s to width, ANSI escape codes don't count towards width
func truncate(s string, w int) string {
	if visualWidth(s) <= w {
		return s
	}
	var out strings.Builder
	width := 0
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\x1b' {
			// copy escape sequence without counting width
			for runes[i] != 'm' {
				out.WriteRune(runes[i])
				i++
			}
			out.WriteRune('m')
			continue
		}
		if width >= w-1 {
			out.WriteString("…")
			out.WriteString("\x1b[0m")
			return out.String()
		}
		out.WriteRune(runes[i])
		width++
	}
	return out.String()
}

// titleBar renders an underline bar for the emphasized todo title
func titleBar(w int) string {
	if w <= 0 {
		return ""
	}
	return "─" + strings.Repeat("─", max(0, w-1))
}

// titleStyle renders the todo title: bold, #ebb58c text color
var titleStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color("#ebb58c"))

// hoverStyle highlights the hovered item in the menu columns
var hoverStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#1c1c1c")).
	Background(lipgloss.Color("#ebb58c"))

// styleTitle renders the todo title with color emphasis so it reads
// larger than the content in the detail view
func styleTitle(s string) string {
	return titleStyle.Render(s)
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
