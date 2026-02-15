package mcp

import (
	"encoding/json"
	"fmt"

	"mcp-gmail-server/internal/llm"
)

type GmailQuery struct {
	Query string `json:"query"`
}

func BuildGmailQuery(
	client llm.Client,
	// client *llm.GroqClient,
	intent string) (string, error) {
	prompt := fmt.Sprintf(`
You are a Gmail search query generator.

Convert the user intent into a valid Gmail search query.

Rules:
- Output ONLY JSON
- No markdown
- Key must be "query"
- Use Gmail search operators
- Prefer broad matching

User intent:
"%s"
`, intent)

	raw, err := client.Extract(prompt)
	if err != nil {
		return "", err
	}

	clean := CleanJSON(raw)

	var q GmailQuery
	if err := json.Unmarshal([]byte(clean), &q); err != nil {
		return "", err
	}

	if q.Query == "" {
		return "", fmt.Errorf("empty gmail query generated")
	}

	return q.Query, nil
}
