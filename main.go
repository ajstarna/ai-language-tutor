package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/charmbracelet/lipgloss"
)

var tutorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
var promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
var toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true)

func main() {

	config := loadConfig()
	fmt.Println(config)

	tutor, err := NewTutor(config)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(promptStyle.Render("> "))
	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "quit" {
			break
		}
		modelOutput, err := tutor.callModel(line)
		if err != nil {
			fmt.Println("error:", err)
			fmt.Print(promptStyle.Render("> "))
			continue
		}
		fmt.Println(tutorStyle.Render(modelOutput))
		fmt.Print(promptStyle.Render("> "))
	}

}
