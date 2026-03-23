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
		switch line {
		case "quit", "/quit":
			return
		case "/help":
			fmt.Println(toolStyle.Render("Available commands:"))
			fmt.Println(toolStyle.Render("  /quiz     — start a vocabulary quiz"))
			fmt.Println(toolStyle.Render("  /endquiz  — exit quiz mode early"))
			fmt.Println(toolStyle.Render("  /config   — change a setting"))
			fmt.Println(toolStyle.Render("  /help     — show this help"))
			fmt.Println(toolStyle.Render("  /quit     — exit the tutor"))
			continue
		case "/endquiz":
			tutor.inQuiz = false
			fmt.Println(toolStyle.Render("Quiz ended."))
			continue
		case "/config":
			newConfig, changed := editConfig(tutor.config)
			if changed {
				tutor.config = newConfig
				tutor.rebuildSystemMessage()
				fmt.Println(toolStyle.Render("Config updated."))
			} else {
				fmt.Println(toolStyle.Render("Cancelled."))
			}
			continue
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
