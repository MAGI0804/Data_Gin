package destination

import "context"

type SOAPPublisher struct{}

func (SOAPPublisher) Code() string {
	return "soap"
}

func (publisher SOAPPublisher) Test(ctx context.Context, cfg Config) error {
	return HTTPPublisher{}.Test(ctx, cfg)
}

func (publisher SOAPPublisher) Publish(ctx context.Context, cfg Config, record CleanRecord) (*PublishResult, error) {
	if _, ok := cfg["headers"]; !ok {
		cfg["headers"] = map[string]interface{}{
			"Content-Type": "text/xml; charset=utf-8",
		}
	}
	return HTTPPublisher{}.Publish(ctx, cfg, record)
}
