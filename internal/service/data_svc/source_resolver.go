package data_svc

import (
	"net/url"
	"strings"

	"gin-biz-web-api/model"
)

var defaultSourceQueryKeys = []string{"source", "data_source", "remark"}

type ResolvedRawSource struct {
	SourceCode       string
	SourceQueryKey   string
	SourceQueryValue string
}

func ResolveRawSource(query url.Values, bodyDataSource, remark string, definitions []model.SourceDefinition) ResolvedRawSource {
	definitionsByCode := make(map[string]model.SourceDefinition, len(definitions))
	for _, definition := range definitions {
		definitionsByCode[definition.Code] = definition
	}

	for _, key := range defaultSourceQueryKeys {
		value := firstQueryValue(query, key)
		if value == "" {
			continue
		}
		definition, ok := definitionsByCode[value]
		if ok && definition.SourceQueryKey != "" {
			configuredValue := firstQueryValue(query, definition.SourceQueryKey)
			if configuredValue != "" {
				return resolvedSource(definition.SourceQueryKey, configuredValue)
			}
		}
	}

	for _, definition := range definitions {
		if !definition.Enabled || definition.SourceQueryKey == "" {
			continue
		}
		value := firstQueryValue(query, definition.SourceQueryKey)
		if value != "" {
			return resolvedSource(definition.SourceQueryKey, value)
		}
	}

	for _, key := range defaultSourceQueryKeys {
		value := firstQueryValue(query, key)
		if value != "" {
			return resolvedSource(key, value)
		}
	}

	if strings.TrimSpace(bodyDataSource) != "" {
		return resolvedSource("body.data_source", bodyDataSource)
	}

	if strings.TrimSpace(remark) != "" {
		return resolvedSource("remark", remark)
	}

	return resolvedSource("default", "unknown")
}

func resolvedSource(key, value string) ResolvedRawSource {
	sourceCode := strings.TrimSpace(value)
	return ResolvedRawSource{
		SourceCode:       sourceCode,
		SourceQueryKey:   key,
		SourceQueryValue: sourceCode,
	}
}

func firstQueryValue(query url.Values, key string) string {
	values, ok := query[key]
	if !ok || len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}
