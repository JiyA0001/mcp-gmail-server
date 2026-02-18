package gmail

import (
	"context"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func NewGmailService(config *oauth2.Config, token *oauth2.Token) (*gmail.Service, error) {
	ctx := context.Background()
	ts := config.TokenSource(ctx, token)
	return gmail.NewService(ctx, option.WithTokenSource(ts))
}
