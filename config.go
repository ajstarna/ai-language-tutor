package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

func loadConfig() Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	configPath := filepath.Join(homeDir, ".goodfriendstutor", "config.json")

	var config Config

	fileBytes, err := os.ReadFile(configPath)

	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return promptConfig(configPath)
		}
		log.Fatalf("failed to read config file %s: %v", configPath, err)
	}

	err = json.Unmarshal(fileBytes, &config)
	if err != nil {
		log.Fatalf("failed to parse config file %s: %v", configPath, err)
	}

	return config
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

	config := Config{
		SourceLanguage: source,
		TargetLanguage: target,
		TutorLanguage:  tutorLang,
		Mode:           mode,
		Strictness:     1,
		Model:          "google/gemini-2.0-flash-001",
	}

	// save for next time
	jsonData, err := json.Marshal(config)
	if err != nil {
		// should not happen
		log.Fatal(err)
	}

	err = os.MkdirAll(filepath.Dir(configPath), 0755)
	if err != nil {
		log.Fatal(err)
	}

	err = os.WriteFile(configPath, jsonData, 0600)
	if err != nil {
		log.Fatal(err)
	}
	return config

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
