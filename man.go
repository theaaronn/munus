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
	modeInsideTodo = iota
	modeTodosList
)

type model struct {
	dirs   []string // meant to follow convention <todoListName>_munus
	dir    string   // current dir
	repo   string   // repo source of todos
	level  int8
	mode   int8 // editing text, moving through todos, inserting a file (future)
	todos  []Todo
	cursor int
}

func (m model) View() tea.View {
	v := tea.NewView(printTodos(m))
	v.AltScreen = true
	return v
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "1":
			m.cursor = 0
			m.dir = m.dirs[0]
			m.changeDir(m.dirs[0])
		case "2":
			m.cursor = 0
			m.dir = m.dirs[1]
			m.changeDir(m.dirs[1])
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "j":
			m.cursor = max(0, m.cursor-1)
		case "down", "k":
			m.cursor = min(len(m.todos)-1, m.cursor+1)
		}
	}
	return m, nil
}

func initialModel() (model, error) {
	dirs, err := loadDirs(".")
	if err != nil {
		return model{}, fmt.Errorf("err while loading dirs from . : %v", err)
	}
	// + In root directory and json format by default
	todos, err := loadTodos(dirs[0], ".json")
	if err != nil {
		return model{}, fmt.Errorf("from initialModel: %v", err)
	}

	return model{
		dirs:   dirs,
		dir:    dirs[0],
		repo:   ".",
		todos:  todos,
		mode:   modeTodosList,
		cursor: 0,
	}, nil
}

// load new todos to m.todos
func (m *model) changeDir(dir string) error {
	todos, err := loadTodos(dir, ".json")
	if err != nil {
		return err
	}
	m.todos = todos
	return nil
}

func loadTodos(dir, ext string) ([]Todo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("at reading todos directory %v: %v", dir, err)
	}
	todos := make([]Todo, 0)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ext) {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("at open file %v: %v", entry, err)
			}
			err = json.Unmarshal(data, &todos)
			if err != nil {
				return nil, fmt.Errorf("at unmarshalling file %v: %v", entry, err)
			}
		}
	}
	return todos, nil
}

// Loads directories ending with _munus to model.dirs
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

func printTodos(m model) string {
	var s strings.Builder
	for idx, todo := range m.todos {
		if m.cursor == idx {
			fmt.Fprint(&s, "[>]")
		} else {
			fmt.Fprint(&s, "[ ]")
		}
		fmt.Fprintln(&s, "Title:", todo.Title, "\ndetail:", todo.Body)
	}
	return s.String()
}

func main() {
	model, err := initialModel()
	if err != nil {
		fmt.Fprintf(os.Stderr, "at main initialModel: %v", err)
		os.Exit(1)
	}
	p := tea.NewProgram(model)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
