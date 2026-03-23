package main

import (
	"fmt"
	"log"

	"github.com/charmbracelet/lipgloss"
	"github.com/chzyer/readline"
)

var tutorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
var promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
var toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true)

func main() {

	config := loadConfig()

	tutor, err := NewTutor(config)
	if err != nil {
		log.Fatal(err)
	}
	defer tutor.Close()

	greeting, err := tutor.callModel("Greet the student warmly and ask what they'd like to work on today.")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(tutorStyle.Render(greeting))

	rl, err := readline.New(promptStyle.Render("> "))
	if err != nil {
		log.Fatal(err)
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			break
		}
		if line == "quit" || line == "/quit" {
			break
		}
		input := line
		if line == "/quiz" {
			input = "Please quiz me on my problem words now."
		}
		modelOutput, err := tutor.callModel(input)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println(tutorStyle.Render(modelOutput))
	}

}
