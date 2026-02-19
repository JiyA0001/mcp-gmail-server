package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"mcp-gmail-server/internal/llm"
	"mcp-gmail-server/internal/models"
)

func BuildPrompt(userIntent string, emails []string) string {
	return fmt.Sprintf(`
	You are a JSON-only information extraction engine.

	User intent:
	"%s"

	Emails:
	%s

	Rules:
	- Extract ONLY relevant information based on the user intent
	- Return ONLY raw JSON
	- Use consistently named keys (lowerCamelCase) across all items
	- If listing emails, use an array under a key like "emails" or "results"
	- DO NOT use markdown or backticks
	- DO NOT add explanation

	Output:
	`, userIntent, strings.Join(emails, "\n\n"))
}

func chunkEmails(emails []string, size int) [][]string {
	var chunks [][]string
	for size < len(emails) {
		emails, chunks = emails[size:], append(chunks, emails[0:size:size])
	}
	chunks = append(chunks, emails)
	return chunks
}

func RunExtraction(
	client llm.Client,
	// client *llm.GroqClient,
	intent string,
	emails []string,
) (models.ExtractedResult, error) {

	chunks := chunkEmails(emails, 20)

	finalResult := make(models.ExtractedResult)

	for _, chunk := range chunks {
		prompt := BuildPrompt(intent, chunk)
		raw, err := client.Extract(prompt)
		if err != nil {
			continue
		}

		clean := CleanJSON(raw)

		var partial map[string]interface{}
		if err := json.Unmarshal([]byte(clean), &partial); err != nil {
			continue
		}

		mergeResults(finalResult, partial)
	}

	return finalResult, nil
}
