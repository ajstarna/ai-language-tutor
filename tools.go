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

type RecordQuizResultArgs struct {
	Term   string `json:"term"`
	Passed bool   `json:"passed"`
}

type StoreUserFactArgs struct {
	Fact string `json:"fact"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
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
		Description: "Retrieve words that are due for review based on spaced repetition scheduling",
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
		Description: "Record whether the user passed or failed a quiz on a given word, used to update the spaced repetition interval",
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

var allTools  = []Tool{getDueWords, recordQuizResult, storeProblemWord, storeUserFact}
var quizTools = []Tool{getDueWords, recordQuizResult, storeUserFact}
