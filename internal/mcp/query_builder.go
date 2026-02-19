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
- Key "query": valid Gmail search operators (e.g. "newer_than:7d")
- Key "limit": integer number of emails to process
  - Default: 10 (if no quantity specified)
  - If user implies "all" or a time range (e.g. "last week", "today"), use a higher limit (e.g. 50, 100, up to 500) to capture everything.
  - Max safety limit: 500

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
	// Increase safety cap to 500
	if limit > 500 {
		limit = 500
	}

	return q.Query, limit, nil
}
