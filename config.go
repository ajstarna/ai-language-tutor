package main

type Mode string

// The mode tells the agent how to behave:
// Guided acts more like an instructor, conversational just chats, mixed with chat but once in a while quiz or something

const (
	ModeGuided         Mode = "guided"
	ModeConversational Mode = "conversational"
	ModeMixed          Mode = "mixed"
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
	return Config{
		SourceLanguage: "English",
		TargetLanguage: "German",
		TutorLanguage:  TutorLanguageSource,
		Mode:           ModeConversational,
		Strictness:     1,
		Model:          "google/gemini-2.0-flash-001",
	}
}
