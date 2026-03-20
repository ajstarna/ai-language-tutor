package main

/*
     {
    "type": "function",
    "function": {
      "name": "store_problem_word",
      "description": "...",
      "parameters": {
        "type": "object",
        "properties": { ... },
        "required": [...]
      }
    }
  }
*/

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type Parameters struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required"`
}

type Tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string     `json:"name"`
		Description string     `json:"description"`
		Parameters  Parameters `json:"parameters"`
	} `json:"function"`
}
