package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
)

//go:embed prompts/*.tmpl
var promptFiles embed.FS

//go:embed curriculum/*.json
var syllabusFile embed.FS

type VocabEntry struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

type CurriculumWeek struct {
	Week          int          `json:"week"`
	CEFR          string       `json:"cefr"`
	Semester      int          `json:"semester"`
	Topic         string       `json:"topic"`
	GrammarFocus  string       `json:"grammar_focus"`
	Competency    string       `json:"competency"`
	Prerequisites []int        `json:"prerequisites"`
	Vocab         []VocabEntry `json:"vocab"`
}

type Syllabus struct {
	Weeks []CurriculumWeek `json:"weeks"`
}

type SyllabusFile struct {
	Syllabus json.RawMessage  `json:"syllabus"`
	Weeks    []CurriculumWeek `json:"weeks"`
}

type syllabusMetadata struct {
	LanguageName string `json:"language_name"`
}

func loadCurriculum(targetLanguage string) []CurriculumWeek {
	entries, err := syllabusFile.ReadDir("curriculum")
	if err != nil {
		log.Printf("warning: could not list curriculum files: %v", err)
		return nil
	}
	target := strings.ToLower(targetLanguage)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := syllabusFile.ReadFile("curriculum/" + entry.Name())
		if err != nil {
			continue
		}
		// check if this file's language matches
		var peek struct {
			Syllabus syllabusMetadata `json:"syllabus"`
		}
		if err := json.Unmarshal(data, &peek); err != nil {
			continue
		}
		if strings.ToLower(peek.Syllabus.LanguageName) != target {
			continue
		}
		var sf SyllabusFile
		if err := json.Unmarshal(data, &sf); err != nil {
			log.Printf("warning: could not parse curriculum %s: %v", entry.Name(), err)
			return nil
		}
		return sf.Weeks
	}
	log.Printf("warning: no curriculum found for %q", targetLanguage)
	return nil
}

type promptData struct {
	TargetLanguage      string
	LanguageInstruction string
	LanguageWarning     string
	Mode                Mode
	Strictness          int
	UserFacts           []string
	GeneralNotes        string
	CurriculumNotes     string
	CurrentWeek         int
}

func buildSystemPrompt(config Config, userFacts []string, currentWeek int) string {
	var languageInstruction, languageWarning string
	switch config.TutorLanguage {
	case TutorLanguageEnglish:
		languageInstruction = "Respond in English (the student's native language)."
		languageWarning = "IMPORTANT: You MUST respond only in English. Do NOT use any other language — not even a single word or phrase."
	case TutorLanguageTarget:
		languageInstruction = fmt.Sprintf("Respond in %s (the language they are learning).", config.TargetLanguage)
		languageWarning = fmt.Sprintf("IMPORTANT: You MUST respond only in %s. Do NOT use any other language — not even a single word or phrase.", config.TargetLanguage)
	case TutorLanguageMixed:
		languageInstruction = fmt.Sprintf("Respond mainly in English but naturally sprinkle in simple %s words and phrases to help the student learn.", config.TargetLanguage)
		languageWarning = fmt.Sprintf("IMPORTANT: Use your judgement on the mix — lean on English for clarity but introduce %s vocabulary and short phrases naturally as the conversation flows.", config.TargetLanguage)
	}

	data := promptData{
		TargetLanguage:      config.TargetLanguage,
		LanguageInstruction: languageInstruction,
		LanguageWarning:     languageWarning,
		Mode:                config.Mode,
		Strictness:          config.Strictness,
		UserFacts:           userFacts,
		GeneralNotes:        loadGeneralNotes(),
		CurriculumNotes:     loadCurriculumNote(config.TargetLanguage, currentWeek),
		CurrentWeek:         currentWeek,
	}

	files := []string{
		"prompts/base.tmpl",
		"prompts/corrections.tmpl",
		"prompts/quiz.tmpl",
		"prompts/tools.tmpl",
		"prompts/curriculum.tmpl",
	}

	var buf strings.Builder
	for _, f := range files {
		tmpl := template.Must(template.ParseFS(promptFiles, f))
		if err := tmpl.Execute(&buf, data); err != nil {
			log.Fatalf("failed to render prompt template %s: %v", f, err)
		}
		buf.WriteString("\n")
	}
	return buf.String()
}

type Tutor struct {
	config     Config
	client     Client
	messages   []ChatMessage
	tools      []Tool
	db         *sql.DB
	userFacts  []string
	inQuiz     bool
	curriculum []CurriculumWeek
}

func NewTutor(config Config) (Tutor, error) {
	key := os.Getenv("OPENROUTER_API_KEY")
	if key == "" {
		return Tutor{}, errors.New("OPENROUTER_API_KEY not set")
	}
	client := Client{
		baseURL: "https://openrouter.ai/api/v1/chat/completions",
		apiKey:  key,
	}

	userFacts := loadUserFacts()
	curriculum := loadCurriculum(config.TargetLanguage)
	db := openDB()
	currentWeek := getCurrentWeek(db, config.TargetLanguage)

	systemPrompt := buildSystemPrompt(config, userFacts, currentWeek)
	messages := []ChatMessage{{Role: "system", Content: systemPrompt}}

	return Tutor{config: config, client: client, messages: messages, tools: allTools, db: db, userFacts: userFacts, curriculum: curriculum}, nil
}

func (t *Tutor) Close() {
	t.db.Close()
}

// rebuildSystemMessage updates the first message (system prompt) with the latest user facts and notes.
func (t *Tutor) rebuildSystemMessage() {
	currentWeek := getCurrentWeek(t.db, t.config.TargetLanguage)
	t.messages[0] = ChatMessage{Role: "system", Content: buildSystemPrompt(t.config, t.userFacts, currentWeek)}
}

const (
	compressThreshold = 90 // compress when message count exceeds this
	keepRecentCount   = 30 // number of recent messages to keep verbatim
)

// maybeCompress summarizes old messages when the conversation gets too long.
// It keeps the system prompt and the most recent messages verbatim, replacing
// everything in between with a fake user/assistant summary pair.
func (t *Tutor) maybeCompress() error {
	if len(t.messages) <= compressThreshold {
		return nil
	}

	// messages to summarize: everything between system prompt and the recent window
	toSummarize := t.messages[1 : len(t.messages)-keepRecentCount]

	// ask the model to summarize the old messages
	summaryRequest := append(toSummarize, ChatMessage{
		Role:    "user",
		Content: "Please summarize the conversation so far in a few concise sentences, focusing on what the student has learned, mistakes they've made, and any personal details mentioned.",
	})
	summary, err := t.client.sendRequest(t.config.Model, summaryRequest, nil)
	if err != nil {
		return err
	}

	// clone recent messages — the slice references the same backing array we're about to overwrite
	recent := append([]ChatMessage{}, t.messages[len(t.messages)-keepRecentCount:]...)
	t.messages = append(
		[]ChatMessage{
			t.messages[0], // system prompt
			{Role: "user", Content: "Summary of earlier conversation:"},
			{Role: "assistant", Content: summary.Content},
		},
		recent...,
	)
	return nil
}

func (t *Tutor) callModel(prompt string) (string, error) {
	if err := t.maybeCompress(); err != nil {
		fmt.Println("warning: could not compress conversation:", err)
	}

	newMessage := ChatMessage{Role: "user", Content: prompt}
	t.messages = append(t.messages, newMessage)

	// The model can return content and tool calls in the same message — it has already
	// composed a reply but also wants to trigger a side effect (e.g. storing a mistake).
	// We save that content as a fallback in case the follow-up response after tool
	// execution comes back empty.
	var fallbackContent string
	for {
		retries := 0  // we retry if the model returns an empty looking results
		var msg ChatMessage
		var err error
		for {
			activeTools := t.tools
		if t.inQuiz {
			activeTools = quizTools
		}
		msg, err = t.client.sendRequest(t.config.Model, t.messages, activeTools)
			//fmt.Printf("msg: %+v\n", msg)
			if err != nil {
				return "", err
			}
			if len(msg.ToolCalls) == 0 && strings.TrimSpace(msg.Content) == "" {
				retries++
				if retries >= 3 {
					return "", errors.New("model returned empty response")
				}
				continue  // retry without appending bad message to history
			}
			break // good message
		}

		t.messages = append(t.messages, msg)

		if len(msg.ToolCalls) == 0 {
			if msg.Content == "" && fallbackContent != "" {
				return fallbackContent, nil
			}
			return msg.Content, nil
		}

		// update fallback whenever the model includes content alongside tool calls
		if msg.Content != "" {
			fallbackContent = msg.Content
		}

		// execute each tool call and append results
		for _, tc := range msg.ToolCalls {
			result := t.executeTool(tc)
			t.messages = append(t.messages, ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
		// loop back and call model again with tool results
	}
}

func (t *Tutor) getCurriculumWeek(week int) *CurriculumWeek {
	for i := range t.curriculum {
		if t.curriculum[i].Week == week {
			return &t.curriculum[i]
		}
	}
	return nil
}

func (t *Tutor) executeTool(toolCall ToolCall) string {
	lang := t.config.TargetLanguage
	switch toolCall.Function.Name {
	case "store_problem_word":
		var args StoreProblemWordArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			log.Printf("store_problem_word: bad arguments: %v", err)
			return "error: could not parse arguments — expected {\"term\": \"...\", \"problem_sentence\": \"...\"}"
		}
		fmt.Println(toolStyle.Render("⟳ Saving problem term: \"" + args.Term + "\""))
		if err := storeMistake(t.db, lang, args.Term, args.ProblemSentence); err != nil {
			log.Printf("store_problem_word: db error: %v", err)
			return "error: failed to store term in database"
		}
		return "stored successfully"
	case "store_taught_words":
		var args StoreTaughtTermsArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			log.Printf("store_taught_words: bad arguments: %v", err)
			return "error: could not parse arguments"
		}
		fmt.Println(toolStyle.Render(fmt.Sprintf("⟳ Saving %d vocab terms...", len(args.Words))))
		if err := storeTaughtTerms(t.db, lang, args.Words); err != nil {
			log.Printf("store_taught_words: db error: %v", err)
			return "error: failed to store taught terms"
		}
		return fmt.Sprintf("saved %d vocab terms", len(args.Words))
	case "get_due_words":
		fmt.Println(toolStyle.Render("⟳ Fetching due terms..."))
		terms, err := getDueTerms(t.db, lang)
		if err != nil {
			log.Printf("get_due_words: db error: %v", err)
			return "error: failed to fetch due terms from database"
		}
		if len(terms) == 0 {
			t.inQuiz = false
			return "no terms due for review — quiz is over"
		}
		t.inQuiz = true
		result, _ := json.Marshal(terms)
		return string(result)
	case "record_quiz_result":
		var args RecordQuizResultArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			log.Printf("record_quiz_result: bad arguments: %v", err)
			return "error: could not parse arguments — expected {\"term\": \"...\", \"passed\": true/false}"
		}
		passed := "✗"
		if args.Passed {
			passed = "✓"
		}
		fmt.Println(toolStyle.Render("⟳ Recording quiz result for: \"" + args.Term + "\" " + passed))
		if err := recordResult(t.db, lang, args.Term, args.Passed); err != nil {
			log.Printf("record_quiz_result: db error: %v", err)
			return "error: failed to record quiz result for term \"" + args.Term + "\""
		}
		return "recorded successfully"
	case "store_user_fact":
		var args StoreUserFactArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			log.Printf("store_user_fact: bad arguments: %v", err)
			return "error: could not parse arguments — expected {\"fact\": \"...\"}"
		}
		fmt.Println(toolStyle.Render("⟳ Remembering: \"" + args.Fact + "\""))
		if err := appendUserFact(args.Fact); err != nil {
			log.Printf("store_user_fact: file error: %v", err)
			return "error: failed to save user fact to profile"
		}
		t.userFacts = append(t.userFacts, args.Fact)
		t.rebuildSystemMessage()
		return "stored successfully"
	case "get_curriculum_week":
		var args GetCurriculumWeekArgs
		json.Unmarshal([]byte(toolCall.Function.Arguments), &args) // args may be empty, that's ok
		week := args.Week
		if week == 0 {
			week = getCurrentWeek(t.db, lang)
		}
		if week == 0 {
			// no week started yet — start week 1
			week = 1
			startWeek(t.db, lang, 1)
		}
		cw := t.getCurriculumWeek(week)
		if cw == nil {
			return fmt.Sprintf("error: week %d not found in curriculum", week)
		}
		fmt.Println(toolStyle.Render(fmt.Sprintf("⟳ Loading curriculum week %d: %s", week, cw.Topic)))
		// include the week's notes alongside the curriculum data
		notes := loadCurriculumNote(lang, week)
		type weekWithNotes struct {
			CurriculumWeek
			Notes string `json:"notes,omitempty"`
		}
		result, _ := json.Marshal(weekWithNotes{*cw, notes})
		return string(result)
	case "complete_curriculum_week":
		currentWeek := getCurrentWeek(t.db, lang)
		if currentWeek == 0 {
			return "error: no curriculum week is in progress"
		}
		cw := t.getCurriculumWeek(currentWeek)
		topicName := fmt.Sprintf("week %d", currentWeek)
		if cw != nil {
			topicName = fmt.Sprintf("week %d: %s", currentWeek, cw.Topic)
		}
		if err := completeWeek(t.db, lang, currentWeek); err != nil {
			log.Printf("complete_curriculum_week: db error: %v", err)
			return "error: failed to complete week"
		}
		nextWeek := currentWeek + 1
		if t.getCurriculumWeek(nextWeek) != nil {
			startWeek(t.db, lang, nextWeek)
			fmt.Println(toolStyle.Render(fmt.Sprintf("⟳ Completed %s — advancing to week %d", topicName, nextWeek)))
			t.rebuildSystemMessage()
			return fmt.Sprintf("completed %s. Now on week %d.", topicName, nextWeek)
		}
		fmt.Println(toolStyle.Render(fmt.Sprintf("⟳ Completed %s — curriculum finished!", topicName)))
		t.rebuildSystemMessage()
		return fmt.Sprintf("completed %s. Congratulations — the curriculum is complete!", topicName)
	case "search_curriculum":
		var args SearchCurriculumArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			log.Printf("search_curriculum: bad arguments: %v", err)
			return "error: could not parse arguments"
		}
		query := strings.ToLower(args.Query)
		fmt.Println(toolStyle.Render("⟳ Searching curriculum for: \"" + args.Query + "\""))
		type match struct {
			Week         int    `json:"week"`
			CEFR         string `json:"cefr"`
			Topic        string `json:"topic"`
			GrammarFocus string `json:"grammar_focus"`
		}
		var matches []match
		for _, cw := range t.curriculum {
			if strings.Contains(strings.ToLower(cw.GrammarFocus), query) ||
				strings.Contains(strings.ToLower(cw.Topic), query) {
				matches = append(matches, match{cw.Week, cw.CEFR, cw.Topic, cw.GrammarFocus})
			}
		}
		if len(matches) == 0 {
			return "no matching weeks found for \"" + args.Query + "\""
		}
		result, _ := json.Marshal(matches)
		return string(result)
	case "get_week_progress":
		currentWeek := getCurrentWeek(t.db, lang)
		if currentWeek == 0 {
			return "error: no curriculum week is in progress"
		}
		cw := t.getCurriculumWeek(currentWeek)
		if cw == nil {
			return fmt.Sprintf("error: week %d not found in curriculum", currentWeek)
		}
		// extract the target-language terms from the vocab list
		var vocabTerms []string
		for _, v := range cw.Vocab {
			vocabTerms = append(vocabTerms, v.Term)
		}
		progress := getWeekProgress(t.db, lang, vocabTerms)
		if progress.LearnedTerms == 0 {
			fmt.Println(toolStyle.Render(fmt.Sprintf("⟳ Week %d: %d terms to review", currentWeek, progress.TotalTerms)))
		} else {
			fmt.Println(toolStyle.Render(fmt.Sprintf("⟳ Week %d: %d/%d terms mastered", currentWeek, progress.LearnedTerms, progress.TotalTerms)))
		}
		result, _ := json.Marshal(progress)
		return string(result)
	case "append_general_note":
		var args AppendNoteArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			log.Printf("append_general_note: bad arguments: %v", err)
			return "error: could not parse arguments"
		}
		fmt.Println(toolStyle.Render("⟳ Noting: \"" + args.Note + "\""))
		if err := appendGeneralNote(args.Note); err != nil {
			log.Printf("append_general_note: file error: %v", err)
			return "error: failed to save note"
		}
		return "noted"
	case "append_curriculum_note":
		var args AppendNoteArgs
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			log.Printf("append_curriculum_note: bad arguments: %v", err)
			return "error: could not parse arguments"
		}
		currentWeek := getCurrentWeek(t.db, lang)
		if currentWeek == 0 {
			return "error: no curriculum week is in progress"
		}
		fmt.Println(toolStyle.Render(fmt.Sprintf("⟳ Week %d note: \"%s\"", currentWeek, args.Note)))
		if err := appendCurriculumNote(lang, currentWeek, args.Note); err != nil {
			log.Printf("append_curriculum_note: file error: %v", err)
			return "error: failed to save curriculum note"
		}
		return "noted"
	default:
		return "error: unknown tool \"" + toolCall.Function.Name + "\""
	}
}
