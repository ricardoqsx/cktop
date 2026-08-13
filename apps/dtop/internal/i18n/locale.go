package i18n

import (
	"os"
	"strings"
)

var localeEnvironment = []string{"DTOP_LOCALE", "LC_ALL", "LC_MESSAGES", "LANG"}

// ResolveLocale returns the first supported locale configured in the process
// environment, or English when none is configured.
func ResolveLocale() string {
	for _, name := range localeEnvironment {
		if value := os.Getenv(name); value != "" {
			return NormalizeLocale(value)
		}
	}
	return "en"
}

// NormalizeLocale reduces supported English and Spanish locale variants to
// their base language. Unsupported and malformed locales use English.
func NormalizeLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if separator := strings.IndexAny(locale, ".@"); separator >= 0 {
		locale = locale[:separator]
	}
	locale = strings.ReplaceAll(locale, "_", "-")

	switch {
	case locale == "es" || strings.HasPrefix(locale, "es-"):
		return "es"
	case locale == "en" || strings.HasPrefix(locale, "en-"):
		return "en"
	default:
		return "en"
	}
}
