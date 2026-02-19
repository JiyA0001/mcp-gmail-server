package gmail

import (
	"encoding/base64"
	"fmt"

	"google.golang.org/api/gmail/v1"
)

// SendEmail sends a plain text email using the Gmail API
func SendEmail(service *gmail.Service, to string, subject string, bodyText string) error {
	msgStr := fmt.Sprintf("To: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s", to, subject, bodyText)
	msg := []byte(msgStr)
	message := &gmail.Message{
		Raw: base64.URLEncoding.EncodeToString(msg),
	}

	_, err := service.Users.Messages.Send("me", message).Do()
	return err
}
