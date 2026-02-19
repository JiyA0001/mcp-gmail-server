package gmail

import (
	"encoding/base64"
	"sync"

	"google.golang.org/api/gmail/v1"
)

type Email struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
	Snippet string `json:"snippet"`
	Body    string `json:"body"`
}

func FetchEmails(service *gmail.Service, query string, limit int) ([]Email, error) {
	if limit <= 0 {
		limit = 10
	}

	// 1. List messages first to get IDs and maintain order
	var messageIDs []string
	var pageToken string

	for len(messageIDs) < limit {
		fetchSize := int64(limit - len(messageIDs))
		if fetchSize > 50 {
			fetchSize = 50
		}

		req := service.Users.Messages.List("me").
			Q(query).
			MaxResults(fetchSize).
			PageToken(pageToken)

		res, err := req.Do()
		if err != nil {
			break
		}

		for _, msg := range res.Messages {
			messageIDs = append(messageIDs, msg.Id)
			if len(messageIDs) >= limit {
				break
			}
		}

		if res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
	}

	// 2. Fetch details concurrently
	count := len(messageIDs)
	if count == 0 {
		return []Email{}, nil
	}

	allEmails := make([]Email, count)
	var wg sync.WaitGroup

	type job struct {
		msgID string
		index int
	}
	jobs := make(chan job, count)

	// Worker
	worker := func() {
		defer wg.Done()
		for j := range jobs {
			msg, err := service.Users.Messages.Get("me", j.msgID).Format("full").Do()
			if err != nil {
				// If error, we might leave empty or handle partial.
				// For now, keep it empty or retry?
				// Let's just continue, the slot will be empty struct.
				// Better: try to populate ID at least.
				allEmails[j.index].ID = j.msgID
				continue
			}

			email := Email{
				ID:      msg.Id,
				Snippet: msg.Snippet,
			}

			for _, h := range msg.Payload.Headers {
				switch h.Name {
				case "From":
					email.From = h.Value
				case "Subject":
					email.Subject = h.Value
				case "Date":
					email.Date = h.Value
				}
			}

			email.Body = extractBody(msg.Payload)
			if email.Body == "" {
				email.Body = msg.Snippet
			}

			// No mutex needed
			allEmails[j.index] = email
		}
	}

	// Start workers
	// Start workers
	numWorkers := 20
	if count < 20 {
		numWorkers = count
	}
	if count > 100 {
		numWorkers = 50 // cap at 50 workers
	}
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go worker()
	}

	// Send jobs
	for i, id := range messageIDs {
		jobs <- job{msgID: id, index: i}
	}
	close(jobs)
	wg.Wait()

	return allEmails, nil
}

func extractBody(part *gmail.MessagePart) string {
	if part == nil {
		return ""
	}

	if part.MimeType == "text/plain" && part.Body != nil && part.Body.Data != "" {
		data, _ := base64.URLEncoding.DecodeString(part.Body.Data)
		return string(data)
	}

	if part.Parts != nil {
		for _, p := range part.Parts {
			if p.MimeType == "text/plain" {
				return extractBody(p)
			}
		}
		// Fallback to html if no text/plain found in immediate children, or recurse deeper
		// Simplified: just return first text/plain found in DFS
		for _, p := range part.Parts {
			res := extractBody(p)
			if res != "" {
				return res
			}
		}
	}

	// If it's HTML, we might want it if no text/plain
	if part.MimeType == "text/html" && part.Body != nil && part.Body.Data != "" {
		data, _ := base64.URLEncoding.DecodeString(part.Body.Data)
		// Strip HTML later? For now, raw HTML is better than nothing
		return string(data)
	}

	return ""
}
