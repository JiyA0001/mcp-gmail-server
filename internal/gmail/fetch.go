package gmail

import (
	"google.golang.org/api/gmail/v1"
)

type Email struct {
	ID      string `json:"id"`
	From    string `json:"from"`
	Subject string `json:"subject"`
	Date    string `json:"date"`
	Snippet string `json:"snippet"`
}

func FetchEmails(service *gmail.Service, query string, maxResults int64) ([]Email, error) {
	var emails []Email

	req := service.Users.Messages.List("me").
		Q(query).
		MaxResults(maxResults)

	res, err := req.Do()
	if err != nil {
		return nil, err
	}

	for _, msg := range res.Messages {
		message, err := service.Users.Messages.Get("me", msg.Id).Do()
		if err != nil {
			continue
		}

		email := Email{
			ID:      msg.Id,
			Snippet: message.Snippet,
		}

		for _, header := range message.Payload.Headers {
			switch header.Name {
			case "From":
				email.From = header.Value
			case "Subject":
				email.Subject = header.Value
			case "Date":
				email.Date = header.Value
			}
		}

		emails = append(emails, email)
	}

	return emails, nil
}

func FetchEmailsPaged(service *gmail.Service, query string, maxPages int) ([]Email, error) {
	var allEmails []Email
	var pageToken string

	for i := 0; i < maxPages; i++ {
		req := service.Users.Messages.List("me").
			Q(query).
			MaxResults(50).
			PageToken(pageToken)

		res, err := req.Do()
		if err != nil {
			return nil, err
		}

		for _, msg := range res.Messages {
			message, err := service.Users.Messages.Get("me", msg.Id).Do()
			if err != nil {
				continue
			}

			email := Email{
				ID:      msg.Id,
				Snippet: message.Snippet,
			}

			for _, h := range message.Payload.Headers {
				switch h.Name {
				case "From":
					email.From = h.Value
				case "Subject":
					email.Subject = h.Value
				case "Date":
					email.Date = h.Value
				}
			}

			allEmails = append(allEmails, email)
		}

		if res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
	}

	return allEmails, nil
}
