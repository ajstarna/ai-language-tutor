package main

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type StoreProblemWordArgs struct {
	Term            string `json:"term"`
	ProblemSentence string `json:"problem_sentence"`
}

type TaughtTerm struct {
	Term       string `json:"term"`
	Definition string `json:"definition"`
}

type StoreTaughtTermsArgs struct {
	Words []TaughtTerm `json:"words"`
}

type RecordQuizResultArgs struct {
	Term   string `json:"term"`
	Passed bool   `json:"passed"`
}

type StoreUserFactArgs struct {
	Fact string `json:"fact"`
}

type SearchCurriculumArgs struct {
	Query string `json:"query"`
}

type AppendNoteArgs struct {
	Note string `json:"note"`
}

type GetCurriculumWeekArgs struct {
	Week int `json:"week,omitempty"`
}

type Property struct {
	Type        string              `json:"type"`
	Description string              `json:"description,omitempty"`
	Items       *Property           `json:"items,omitempty"`
	Properties  map[string]Property `json:"properties,omitempty"`
	Required    []string            `json:"required,omitempty"`
}

type Parameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type ToolFunction struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Parameters  Parameters `json:"parameters"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// Available tools
var getDueWords = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "get_due_words",
		Description: "Retrieve terms that are due for review based on spaced repetition scheduling",
		Parameters: Parameters{
			Type:       "object",
			Properties: map[string]Property{},
			Required:   []string{},
		},
	},
}

var recordQuizResult = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "record_quiz_result",
		Description: "Record whether the user passed or failed a quiz on a given term, used to update the spaced repetition interval",
		Parameters: Parameters{
			Type: "object",
			Properties: map[string]Property{
				"term": {
					Type:        "string",
					Description: "the term that was quizzed",
				},
				"passed": {
					Type:        "boolean",
					Description: "whether the user answered correctly",
				},
			},
			Required: []string{"term", "passed"},
		},
	},
}

var storeProblemWord = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "store_problem_word",
		Description: "Call this when the student makes a grammar or vocabulary mistake focused on a specific term. This stores it in the DB for later quizzing",
		Parameters: Parameters{
			Type: "object",
			Properties: map[string]Property{
				"term": Property{
					Type:        "string",
					Description: "the correct form of the term the user used incorrectly",
				},
				"problem_sentence": Property{
					Type:        "string",
					Description: "the sentence in which the user made the mistake",
				},
			},
			Required: []string{"term", "problem_sentence"},
		},
	},
}

var storeUserFact = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "store_user_fact",
		Description: "Store a personal fact about the student for future conversations, such as their name, job, hobbies, family, or interests. Do NOT use this for language mistakes — those go to store_problem_word.",
		Parameters: Parameters{
			Type: "object",
			Properties: map[string]Property{
				"fact": {
					Type:        "string",
					Description: "a concise fact about the student, e.g. 'has a baby', 'works as a firefighter'",
				},
			},
			Required: []string{"fact"},
		},
	},
}

var storeTaughtTermsTool = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "store_taught_words",
		Description: "After teaching the student new vocabulary (e.g. during a topic), call this to save the terms for future quizzing via spaced repetition.",
		Parameters: Parameters{
			Type: "object",
			Properties: map[string]Property{
				"words": {
					Type:        "array",
					Description: "list of terms that were taught",
					Items: &Property{
						Type: "object",
						Properties: map[string]Property{
							"term": {
								Type:        "string",
								Description: "the term in the target language",
							},
							"definition": {
								Type:        "string",
								Description: "short definition or translation in the student's native language",
							},
						},
						Required: []string{"term", "definition"},
					},
				},
			},
			Required: []string{"words"},
		},
	},
}

var getCurriculumWeekTool = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "get_curriculum_week",
		Description: "Load the curriculum data for a specific week (topic, grammar focus, vocabulary, competency goal). If no week is specified, loads the student's current week.",
		Parameters: Parameters{
			Type: "object",
			Properties: map[string]Property{
				"week": {
					Type:        "integer",
					Description: "week number to load (optional — defaults to current week)",
				},
			},
			Required: []string{},
		},
	},
}

var completeCurriculumWeekTool = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "complete_curriculum_week",
		Description: "Mark the current curriculum week as completed and advance to the next week. Only call this when the student has demonstrated sufficient mastery of the week's material.",
		Parameters: Parameters{
			Type:       "object",
			Properties: map[string]Property{},
			Required:   []string{},
		},
	},
}

var appendGeneralNoteTool = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "append_general_note",
		Description: "Save an observation about the student's language ability or patterns. Use this during free conversation when you notice something worth remembering across sessions (e.g. 'avoids subjunctive', 'comfortable with past tense', 'tends to forget article genders').",
		Parameters: Parameters{
			Type: "object",
			Properties: map[string]Property{
				"note": {
					Type:        "string",
					Description: "a concise observation about the student's language skills",
				},
			},
			Required: []string{"note"},
		},
	},
}

var appendCurriculumNoteTool = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "append_curriculum_note",
		Description: "Save a progress observation for the current curriculum week. Use this when the student demonstrates understanding or struggle with the week's material (e.g. 'understands werden + infinitive for future tense', 'struggling with dative prepositions, keeps using accusative after mit').",
		Parameters: Parameters{
			Type: "object",
			Properties: map[string]Property{
				"note": {
					Type:        "string",
					Description: "a concise observation about the student's progress on this week's material",
				},
			},
			Required: []string{"note"},
		},
	},
}

var searchCurriculumTool = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "search_curriculum",
		Description: "Search the curriculum for weeks that cover a specific grammar concept or topic. Use this when the student is struggling with something and you want to find where it was originally taught.",
		Parameters: Parameters{
			Type: "object",
			Properties: map[string]Property{
				"query": {
					Type:        "string",
					Description: "the concept to search for, e.g. 'dative prepositions', 'past tense', 'modal verbs'",
				},
			},
			Required: []string{"query"},
		},
	},
}

var getWeekProgressTool = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "get_week_progress",
		Description: "Check the student's progress on the current curriculum week's vocabulary. Returns how many terms have been stored, and how many are 'learned' (passed quiz at least twice). Use this to decide if the student is ready to advance.",
		Parameters: Parameters{
			Type:       "object",
			Properties: map[string]Property{},
			Required:   []string{},
		},
	},
}

var allTools = []Tool{
	getDueWords, recordQuizResult, storeProblemWord, storeTaughtTermsTool,
	storeUserFact, getCurriculumWeekTool, completeCurriculumWeekTool,
	getWeekProgressTool, searchCurriculumTool, appendGeneralNoteTool, appendCurriculumNoteTool,
}
var quizTools = []Tool{getDueWords, recordQuizResult, storeUserFact}
