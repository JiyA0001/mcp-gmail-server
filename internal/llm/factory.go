package llm

import (
	"fmt"
	"os"
)

func NewLLM() (Client, error) {
	provider := os.Getenv("LLM_PROVIDER")

	switch provider {

	case "claude":
		key := os.Getenv("CLAUDE_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("CLAUDE_API_KEY not set")
		}
		return NewClaudeClient(key), nil

	case "groq":
		key := os.Getenv("GROQ_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("GROQ_API_KEY not set")
		}
		return NewGroqClient(key), nil

	case "gemini":
		key := os.Getenv("GEMINI_API_KEY")
		if key == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY not set")
		}
		return NewGeminiClient(key), nil

	default:
		return nil, fmt.Errorf("unsupported LLM_PROVIDER: %s", provider)
	}
}
