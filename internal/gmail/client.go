package gmail

import (
	"context"

	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func NewGmailService(ctx context.Context, oauthConfig *oauth2.Config, token *oauth2.Token) (*gmail.Service, error) {
	client := oauthConfig.Client(ctx, token)
	return gmail.NewService(ctx, option.WithHTTPClient(client))
}
