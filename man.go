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
	files, err := loadTodos(".", "json")
	if err != nil {
		fmt.Printf("at load: %v", err)
	}

	for _, file := range files {
		data, err := os.Open(file)
		if err != nil {
			fmt.Printf("at open file %v: %v", file, err)
		}
		dec := json.NewDecoder(data)
		for dec.More() {
			var todo Todo
			err := dec.Decode(&todo)
			if err != nil {
				fmt.Printf("at decode: %v", err)
				return
			}
			model.todos = append(model.todos, todo)
		}
	}
	for _, todo := range  model.todos {
		fmt.Println(todo.Title, todo.Body)
	}
}

func loadTodos(dir, ext string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ext) {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files, nil
}

