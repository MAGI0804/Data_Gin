package source

import (
	"context"
	"errors"
)

type WebhookConnector struct{}

func (WebhookConnector) Code() string {
	return "webhook"
}

func (WebhookConnector) Test(ctx context.Context, cfg Config) error {
	return ctx.Err()
}

func (WebhookConnector) Fetch(ctx context.Context, cfg Config, cursor FetchCursor) (*FetchResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, errors.New("webhook source does not support active fetch")
}
