package mcp

import "strings"

func CleanJSON(raw string) string {
	raw = strings.TrimSpace(raw)

	start := strings.IndexAny(raw, "{[")
	if start == -1 {
		return raw // fallback
	}

	end := strings.LastIndexAny(raw, "}]")
	if end == -1 || end < start {
		return raw // fallback
	}

	return raw[start : end+1]
}
