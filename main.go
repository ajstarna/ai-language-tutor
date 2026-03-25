package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/chzyer/readline"
)

var modelPresets = []struct {
	label string
	id    string
}{
	{"Haiku (fast, cheap)", "anthropic/claude-haiku-4-5-20251001"},
	{"Sonnet (balanced)", "anthropic/claude-sonnet-4-6"},
	{"GPT-5.4 Mini (fast, cheap)", "openai/gpt-5.4-mini"},
}

func pickModel(rl *readline.Instance, current string) string {
	fmt.Printf("Current model: %s\n", current)
	for i, p := range modelPresets {
		fmt.Printf("  %d. %s (%s)\n", i+1, p.label, p.id)
	}
	fmt.Println("  Or type a model ID directly. Enter to cancel.")

	rl.SetPrompt(promptStyle.Render("model> "))
	defer rl.SetPrompt(promptStyle.Render("> "))

	line, err := rl.Readline()
	if err != nil {
		return ""
	}
	input := strings.TrimSpace(line)
	if input == "" {
		return ""
	}
	if n, err := strconv.Atoi(input); err == nil && n >= 1 && n <= len(modelPresets) {
		return modelPresets[n-1].id
	}
	return input // custom model ID
}

var tutorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("183"))
var promptStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("117"))
var toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Italic(true)

var bannerStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("117")).
	Bold(true)

var subtitleStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("242"))

const banner = `
  ╔═══════════════════════════════════════╗
  ║         Good Friends Tutor            ║
  ╚═══════════════════════════════════════╝`

func main() {
	fmt.Println(bannerStyle.Render(banner))

	config := loadConfig()
	fmt.Println(subtitleStyle.Render(fmt.Sprintf("  Learning: %s  |  mode: %s  |  model: %s  |  strictness: %d",
		config.TargetLanguage, config.Mode, config.Model, config.Strictness)))
	fmt.Println(subtitleStyle.Render("  Try /topic to learn new vocabulary, /quiz to review, /help for more"))
	fmt.Println()

	tutor, err := NewTutor(config)
	if err != nil {
		log.Fatal(err)
	}
	defer tutor.Close()

	var greetingPrompt string
	if config.Mode == ModeCurriculum {
		greetingPrompt = "Greet the student warmly. Then call get_curriculum_week to load the current week's material and begin teaching."
	} else {
		greetingPrompt = "Greet the student warmly. Suggest they pick a topic to learn new vocabulary (they can use /topic), or just chat freely. Keep it short."
	}
	greeting, err := tutor.callModel(greetingPrompt)
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
			fmt.Println(toolStyle.Render("  /quiz          — start a vocabulary quiz"))
			fmt.Println(toolStyle.Render("  /endquiz       — exit quiz mode early"))
			fmt.Println(toolStyle.Render("  /topic [theme] — learn vocab for a topic (random if no theme)"))
			fmt.Println(toolStyle.Render("  /week          — show current curriculum week"))
			fmt.Println(toolStyle.Render("  /model         — switch the AI model"))
			fmt.Println(toolStyle.Render("  /config        — change a setting"))
			fmt.Println(toolStyle.Render("  /help          — show this help"))
			fmt.Println(toolStyle.Render("  /quit          — exit the tutor"))
			continue
		case "/endquiz":
			tutor.inQuiz = false
			fmt.Println(toolStyle.Render("Quiz ended."))
			continue
		case "/week":
			week := getCurrentWeek(tutor.db, tutor.config.TargetLanguage)
			if week == 0 {
				fmt.Println(toolStyle.Render("No curriculum week started yet. Use curriculum mode to begin."))
			} else {
				cw := tutor.getCurriculumWeek(week)
				if cw != nil {
					fmt.Println(toolStyle.Render(fmt.Sprintf("  Week %d (%s): %s", week, cw.CEFR, cw.Topic)))
					fmt.Println(toolStyle.Render(fmt.Sprintf("  Grammar: %s", cw.GrammarFocus)))
					fmt.Println(toolStyle.Render(fmt.Sprintf("  Goal: %s", cw.Competency)))
				} else {
					fmt.Println(toolStyle.Render(fmt.Sprintf("  Week %d (not found in curriculum)", week)))
				}
			}
			continue
		case "/model":
			newModel := pickModel(rl, tutor.config.Model)
			if newModel != "" && newModel != tutor.config.Model {
				tutor.config.Model = newModel
				if err := saveConfig(tutor.config); err != nil {
					fmt.Println("warning: could not save config:", err)
				}
				fmt.Println(toolStyle.Render("Model set to " + newModel))
			} else {
				fmt.Println(toolStyle.Render("Cancelled."))
			}
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
			input = "Please quiz me on my terms now."
		} else if line == "/topic" {
			input = "Pick an interesting, everyday topic and teach me about 10 useful terms for it. Present them with definitions, then save them with store_taught_words. After that, let's have a conversation using them."
		} else if strings.HasPrefix(line, "/topic ") {
			topic := strings.TrimPrefix(line, "/topic ")
			input = fmt.Sprintf("Teach me about 10 useful terms related to: %s. Present them with definitions, then save them with store_taught_words. After that, let's have a conversation using them.", topic)
		}
		modelOutput, err := tutor.callModel(input)
		if err != nil {
			fmt.Println("error:", err)
			continue
		}
		fmt.Println(tutorStyle.Render(modelOutput))
	}

}
