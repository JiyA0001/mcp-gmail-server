package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ClaudeClient struct {
	ApiKey string
	Model  string
}

func NewClaudeClient(apiKey string) *ClaudeClient {
	return &ClaudeClient{
		ApiKey: apiKey,
		Model:  "claude-opus-4-6",
	}
}

func (c *ClaudeClient) Extract(prompt string) (string, error) {
	url := "https://api.anthropic.com/v1/messages"

	payload := map[string]interface{}{
		"model":      c.Model,
		"max_tokens": 1024,
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("x-api-key", c.ApiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Claude API error (%d): %s", resp.StatusCode, raw)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}

	content := res["content"].([]interface{})[0].(map[string]interface{})["text"].(string)
	return content, nil
}
