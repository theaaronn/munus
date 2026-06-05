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
	repo  string
	level int8
	mode  int8 // editing text, moving through todos, inserting a file
	todos []Todo
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
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func initialModel() (model, error) {
	// + In root directory and json format by default
	todos, err := loadTodos(".", ".json")
	if err != nil {
		return model{}, fmt.Errorf("from initialModel: %v", err)
	}

	return model{
		repo:  ".",
		todos: todos,
		mode:  modeTodosList,
	}, nil
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
			var todo Todo
			err = json.Unmarshal(data, &todo)
			if err != nil {
				return nil, fmt.Errorf("at unmarshalling file %v: %v", entry, err)
			}
			todos = append(todos, todo)
		}
	}
	return todos, nil
}

func printTodos(m model) string {
	var s strings.Builder
	for _, todo := range m.todos {
		s.WriteString(fmt.Sprintln("Title:", todo.Title, "\ndetail:", todo.Body))
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
