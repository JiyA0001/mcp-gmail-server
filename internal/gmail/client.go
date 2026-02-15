package gmail

import (
	"context"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func NewGmailService(token *oauth2.Token) (*gmail.Service, error) {
	ctx := context.Background()
	return gmail.NewService(ctx, option.WithTokenSource(oauth2.StaticTokenSource(token)))
}
