package i18n

import "testing"

func TestResolveLocalePrecedence(t *testing.T) {
	t.Setenv("LANG", "es_MX.UTF-8")
	t.Setenv("LC_MESSAGES", "en_GB.UTF-8")
	t.Setenv("LC_ALL", "es_ES.UTF-8")
	t.Setenv("DTOP_LOCALE", "en_US.UTF-8")

	if got := ResolveLocale(); got != "en" {
		t.Fatalf("expected DTOP_LOCALE to win, got %q", got)
	}

	t.Setenv("DTOP_LOCALE", "")
	if got := ResolveLocale(); got != "es" {
		t.Fatalf("expected LC_ALL to win, got %q", got)
	}

	t.Setenv("LC_ALL", "")
	if got := ResolveLocale(); got != "en" {
		t.Fatalf("expected LC_MESSAGES to win, got %q", got)
	}

	t.Setenv("LC_MESSAGES", "")
	if got := ResolveLocale(); got != "es" {
		t.Fatalf("expected LANG to win, got %q", got)
	}
}

func TestNormalizeLocale(t *testing.T) {
	tests := map[string]string{
		"es":             "es",
		"es_ES.UTF-8":    "es",
		"es-MX":          "es",
		"ES_ar@euro":     "es",
		"en":             "en",
		"en_US.UTF-8":    "en",
		"en-GB":          "en",
		"C":              "en",
		"fr_FR.UTF-8":    "en",
		"not-a-locale":   "en",
		"  es_CL.UTF-8 ": "es",
	}

	for input, expected := range tests {
		t.Run(input, func(t *testing.T) {
			if got := NormalizeLocale(input); got != expected {
				t.Fatalf("NormalizeLocale(%q) = %q, want %q", input, got, expected)
			}
		})
	}
}

func TestResolveLocaleDefaultsToEnglish(t *testing.T) {
	for _, name := range localeEnvironment {
		t.Setenv(name, "")
	}
	if got := ResolveLocale(); got != "en" {
		t.Fatalf("expected English default, got %q", got)
	}
}

func TestResolveLocaleDoesNotDependOnAmbientLocale(t *testing.T) {
	for _, name := range localeEnvironment {
		t.Setenv(name, "")
	}
	t.Setenv("LANG", "es_AR.UTF-8")
	if got := NewFromEnvironment().Text(MessageTabContainers); got != "Contenedores" {
		t.Fatalf("environment translator = %q, want Spanish", got)
	}
	t.Setenv("DTOP_LOCALE", "en")
	if got := NewFromEnvironment().Text(MessageTabContainers); got != "Containers" {
		t.Fatalf("DTOP_LOCALE override = %q, want English", got)
	}
}
