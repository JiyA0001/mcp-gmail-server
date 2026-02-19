package mcp

import (
	"encoding/json"
	"fmt"

	"mcp-gmail-server/internal/llm"
)

type GmailQuery struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func BuildGmailQuery(
	client llm.Client,
	// client *llm.GroqClient,
	intent string) (string, int, error) {
	prompt := fmt.Sprintf(`
You are a Gmail search query generator.

Convert the user intent into a valid Gmail search query + limit.

Rules:
- Output ONLY JSON
- No markdown
- Key "query": valid Gmail search operators (e.g. "is:unread label:inbox")
- Key "limit": integer number of emails to process (default: 10, max: 50)
- Prefer broad matching

User intent:
"%s"
`, intent)

	raw, err := client.Extract(prompt)
	if err != nil {
		return "", 0, err
	}

	clean := CleanJSON(raw)

	var q GmailQuery
	if err := json.Unmarshal([]byte(clean), &q); err != nil {
		return "", 0, err
	}

	if q.Query == "" {
		return "", 0, fmt.Errorf("empty gmail query generated")
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	return q.Query, limit, nil
}
