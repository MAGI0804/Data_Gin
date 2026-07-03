package source

import "context"

type Config map[string]interface{}

type FetchCursor struct {
	Value string
}

type FetchResult struct {
	Records []map[string]interface{}
	Cursor  FetchCursor
}

type Connector interface {
	Code() string
	Test(ctx context.Context, cfg Config) error
	Fetch(ctx context.Context, cfg Config, cursor FetchCursor) (*FetchResult, error)
}

func Builtins() map[string]Connector {
	connectors := []Connector{
		WebhookConnector{},
		APIConnector{},
		DatabaseConnector{},
	}

	registry := make(map[string]Connector, len(connectors))
	for _, connector := range connectors {
		registry[connector.Code()] = connector
	}

	return registry
}

func StringValue(cfg Config, key string) string {
	value, ok := cfg[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
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
