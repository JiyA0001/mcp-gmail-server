package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GroqClient struct {
	ApiKey string
	Model  string
}

func NewGroqClient(apiKey string) *GroqClient {
	return &GroqClient{
		ApiKey: apiKey,
		Model:  "openai/gpt-oss-120b",
	}
}

func (g *GroqClient) Extract(prompt string) (string, error) {
	url := "https://api.groq.com/openai/v1/chat/completions"

	payload := map[string]interface{}{
		"model": g.Model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a JSON-only engine. Do not output markdown.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0,
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+g.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 🔴 Read raw body FIRST
	bodyBytes, _ := io.ReadAll(resp.Body)

	// 🔴 Handle non-200 responses
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Groq API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	// 🔴 Parse safely
	var res map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return "", err
	}

	choicesRaw, ok := res["choices"]
	if !ok {
		return "", fmt.Errorf("Groq response missing choices: %s", string(bodyBytes))
	}

	choices, ok := choicesRaw.([]interface{})
	if !ok || len(choices) == 0 {
		return "", fmt.Errorf("Groq choices invalid: %s", string(bodyBytes))
	}

	choice := choices[0].(map[string]interface{})
	message := choice["message"].(map[string]interface{})
	content := message["content"].(string)

	return content, nil
}
