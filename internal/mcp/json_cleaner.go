package mcp

import "strings"

func CleanJSON(raw string) string {
	raw = strings.TrimSpace(raw)

	// Remove markdown ```json and ```
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")

	return strings.TrimSpace(raw)
}
