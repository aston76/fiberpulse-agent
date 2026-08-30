package localization

import (
	"context"
	"strings"
)

const Default = "en"

var supported = map[string]struct{}{
	"en": {}, "fr": {}, "de": {}, "es": {}, "pt-BR": {}, "it": {}, "hi": {},
}

func Normalize(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "_", "-"))
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "fr"):
		return "fr"
	case strings.HasPrefix(lower, "de"):
		return "de"
	case strings.HasPrefix(lower, "es"):
		return "es"
	case strings.HasPrefix(lower, "pt"):
		return "pt-BR"
	case strings.HasPrefix(lower, "it"):
		return "it"
	case strings.HasPrefix(lower, "hi"):
		return "hi"
	default:
		return Default
	}
}

func Supported(value string) bool {
	_, ok := supported[value]
	return ok
}

type contextKey struct{}

func WithLanguage(ctx context.Context, value string) context.Context {
	return context.WithValue(ctx, contextKey{}, Normalize(value))
}

func FromContext(ctx context.Context) string {
	if value, ok := ctx.Value(contextKey{}).(string); ok {
		return Normalize(value)
	}
	return Default
}
