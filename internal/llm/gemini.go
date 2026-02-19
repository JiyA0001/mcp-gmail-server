package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GeminiClient struct {
	ApiKey string
}

func NewGeminiClient(apiKey string) *GeminiClient {
	return &GeminiClient{ApiKey: apiKey}
}

func (g *GeminiClient) Extract(prompt string) (string, error) {
	// Use a stable model version
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1/models/gemini-2.5-flash-lite:generateContent?key=%s",
		g.ApiKey,
	)

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
	}

	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 🔴 READ RAW RESPONSE FIRST
	bodyBytes, _ := io.ReadAll(resp.Body)

	// 🔴 PRINT RAW RESPONSE (VERY IMPORTANT)
	fmt.Println("========== RAW GEMINI RESPONSE ==========")
	fmt.Println(string(bodyBytes))
	fmt.Println("=========================================")

	// 🔴 HANDLE NON-200 STATUS
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, string(bodyBytes))
	}

	// 🔴 NOW PARSE JSON SAFELY
	var res map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return "", err
	}

	candidates, ok := res["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return "", fmt.Errorf("no candidates in Gemini response")
	}

	candidate := candidates[0].(map[string]interface{})
	content := candidate["content"].(map[string]interface{})
	parts := content["parts"].([]interface{})
	text := parts[0].(map[string]interface{})["text"].(string)

	return text, nil
}
