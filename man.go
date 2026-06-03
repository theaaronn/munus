package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Todo struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type model struct {
	repo  string
	level int8
	mode  int8 // editing text, moving through todos, inserting a file
	todos []Todo
}

func main() {
	var model model
	err := loadTodos(".", "json", &model)
	if err != nil {
		fmt.Printf("at load: %v", err)
		return
	}
	for _, todo := range model.todos {
		fmt.Println("Title:", todo.Title, "\ndetail:", todo.Body)
	}
}

func loadTodos(dir, ext string, model *model) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ext) {
			data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				return fmt.Errorf("at open file %v: %v", entry, err)
			}
			var todo Todo
			err = json.Unmarshal(data, &todo)
			if err != nil {
				return fmt.Errorf("at unmarshalling file %v: %v", entry, err)
			}
			model.todos = append(model.todos, todo)
		}
	}
	return nil
}
