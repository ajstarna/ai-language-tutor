package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

type Mode string

// The mode tells the agent how to behave:
// Guided acts more like an instructor, conversational just chats, mixed with chat but once in a while quiz or something

const (
	ModeConversational Mode = "conversational"
	ModeCurriculum     Mode = "curriculum"
)

type TutorLanguage string

const (
	TutorLanguageSource TutorLanguage = "source"
	TutorLanguageTarget TutorLanguage = "target"
	TutorLanguageMixed  TutorLanguage = "mixed"
)

type Config struct {
	SourceLanguage string
	TargetLanguage string
	TutorLanguage  TutorLanguage
	Mode           Mode
	Strictness     int // e.g. 1-3
	Model          string
}

func configPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	return filepath.Join(homeDir, ".goodfriendstutor", "config.json")
}

func loadConfig() Config {
	path := configPath()

	var config Config

	fileBytes, err := os.ReadFile(path)

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return promptConfig(path)
		}
		log.Fatalf("failed to read config file %s: %v", path, err)
	}

	err = json.Unmarshal(fileBytes, &config)
	if err != nil {
		log.Fatalf("failed to parse config file %s: %v", path, err)
	}

	return config
}

func saveConfig(config Config) error {
	path := configPath()
	jsonData, err := json.Marshal(config)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, jsonData, 0600)
}

// TODO: improve all this
func promptConfig(configPath string) Config {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("What language do you currently speak")
	scanner.Scan()
	source := scanner.Text()

	fmt.Println("What language do you want to learn?")
	scanner.Scan()
	target := scanner.Text()

	var tutorLang TutorLanguage
	var err error
	for {
		fmt.Println("What language should your tutor speak: ('source', 'target', 'mixed'):")
		scanner.Scan()
		tutorLang, err = parseTutorLanguage(scanner.Text())
		if err == nil {
			break
		}
	}

	var mode Mode
	for {
		fmt.Println("What mode: ('conversational', 'curriculum'):")
		scanner.Scan()
		mode, err = parseMode(scanner.Text())
		if err == nil {
			break
		}
	}

	var strictness int
	for {
		fmt.Println("Strictness level (1=relaxed, 2=moderate, 3=strict):")
		scanner.Scan()
		strictness, err = strconv.Atoi(scanner.Text())
		if err == nil && strictness >= 1 && strictness <= 3 {
			break
		}
		fmt.Println("Please enter 1, 2, or 3.")
	}

	config := Config{
		SourceLanguage: source,
		TargetLanguage: target,
		TutorLanguage:  tutorLang,
		Mode:           mode,
		Strictness:     strictness,
		Model:          "google/gemini-2.0-flash-001",
	}

	// save for next time
	if err := saveConfig(config); err != nil {
		log.Fatal(err)
	}
	return config

}

func editConfig(config Config) (Config, bool) {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Which setting? (tutor_language, mode, strictness, model) or 'cancel':")
	scanner.Scan()
	field := scanner.Text()

	switch field {
	case "tutor_language":
		for {
			fmt.Printf("Tutor language (currently %q) — enter 'source', 'target', or 'mixed':\n", config.TutorLanguage)
			scanner.Scan()
			tl, err := parseTutorLanguage(scanner.Text())
			if err == nil {
				config.TutorLanguage = tl
				break
			}
		}
	case "mode":
		for {
			fmt.Printf("Mode (currently %q) — enter 'conversational' or 'curriculum':\n", config.Mode)
			scanner.Scan()
			m, err := parseMode(scanner.Text())
			if err == nil {
				config.Mode = m
				break
			}
		}
	case "strictness":
		for {
			fmt.Printf("Strictness (currently %d) — enter 1, 2, or 3:\n", config.Strictness)
			scanner.Scan()
			s, err := strconv.Atoi(scanner.Text())
			if err == nil && s >= 1 && s <= 3 {
				config.Strictness = s
				break
			}
			fmt.Println("Please enter 1, 2, or 3.")
		}
	case "model":
		fmt.Printf("Model (currently %q) — enter new model ID:\n", config.Model)
		scanner.Scan()
		config.Model = scanner.Text()
	default:
		return config, false
	}

	if err := saveConfig(config); err != nil {
		fmt.Println("warning: could not save config:", err)
	}
	return config, true
}

func parseMode(s string) (Mode, error) {
	switch Mode(s) {
	case ModeConversational, ModeCurriculum:
		return Mode(s), nil
	default:
		return "", fmt.Errorf("invalid mode %q", s)
	}
}

func parseTutorLanguage(s string) (TutorLanguage, error) {
	switch TutorLanguage(s) {
	case TutorLanguageSource, TutorLanguageTarget, TutorLanguageMixed:
		return TutorLanguage(s), nil
	default:
		return "", fmt.Errorf("invalid tutor language %q", s)
	}
}
