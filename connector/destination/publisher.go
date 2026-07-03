package destination

import "context"

type Config map[string]interface{}

type CleanRecord struct {
	ID          uint
	BusinessKey string
	Content     map[string]interface{}
}

type PublishResult struct {
	Success      bool
	HTTPStatus   int
	RequestBody  string
	ResponseBody string
	ErrorMessage string
}

type Publisher interface {
	Code() string
	Test(ctx context.Context, cfg Config) error
	Publish(ctx context.Context, cfg Config, record CleanRecord) (*PublishResult, error)
}

func Builtins() map[string]Publisher {
	publishers := []Publisher{
		HTTPPublisher{},
		SOAPPublisher{},
	}

	registry := make(map[string]Publisher, len(publishers))
	for _, publisher := range publishers {
		registry[publisher.Code()] = publisher
	}

	return registry
}

func StringValue(cfg Config, key string) string {
	value, ok := cfg[key]
	if !ok || value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

func IntValue(cfg Config, key string, fallback int) int {
	value, ok := cfg[key]
	if !ok || value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return fallback
	}
}
