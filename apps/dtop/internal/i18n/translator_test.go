package i18n

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	sharedui "github.com/ricardoqsx/cktop/libs/tui"
)

func TestSharedShellEnglishAndSpanish(t *testing.T) {
	tests := []struct {
		locale   string
		status   string
		footer   string
		help     string
		fallback string
	}{
		{locale: "en", status: "LOADING", footer: "[Tab] next", help: "Help", fallback: "Overview"},
		{locale: "es", status: "CARGANDO", footer: "[Tab] siguiente", help: "Ayuda", fallback: "Resumen"},
	}

	for _, test := range tests {
		t.Run(test.locale, func(t *testing.T) {
			model := sharedui.NewShell(sharedui.ShellOptions{
				Localizer: New(test.locale),
				Views:     []sharedui.View{{Title: "Containers", Status: sharedui.StatusLoading}},
			})
			view := model.View()
			for _, expected := range []string{test.status, test.footer} {
				if !strings.Contains(view, expected) {
					t.Fatalf("expected shell to contain %q, got %q", expected, view)
				}
			}

			updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
			if view := updated.View(); !strings.Contains(view, test.help) {
				t.Fatalf("expected help to contain %q, got %q", test.help, view)
			}

			fallback := sharedui.NewShell(sharedui.ShellOptions{Localizer: New(test.locale)}).View()
			if !strings.Contains(fallback, test.fallback) {
				t.Fatalf("expected fallback to contain %q, got %q", test.fallback, fallback)
			}
		})
	}
}

func TestTranslatorFallsBackToEnglish(t *testing.T) {
	translator := New("fr-FR")
	if got := translator.Text(sharedui.MessageShellHelpTitle); got != "Help" {
		t.Fatalf("expected unsupported locale to use English, got %q", got)
	}

	translator = &Translator{locale: "es", messages: map[string]message{}}
	if got := translator.Text(sharedui.MessageShellStatusLoading); got != "loading" {
		t.Fatalf("expected missing Spanish message to use English, got %q", got)
	}
	if got := translator.Text("missing.message", "ignored"); got != "missing.message" {
		t.Fatalf("expected unknown message ID as final fallback, got %q", got)
	}
}

func TestSpanishConfirmationDocumentsAcceptedKeys(t *testing.T) {
	got := New("es").Text(MessageConfirmControls)
	if !strings.Contains(got, "[y/N]") {
		t.Fatalf("Spanish confirmation controls = %q, want accepted y/N keys", got)
	}
}

func TestAdvancedMessagesAreLocalizedAndShowExactCommand(t *testing.T) {
	for locale, expected := range map[string][]string{
		"en": {"Advanced", "Delete unused Docker data", "Command: [docker system prune --all --force]"},
		"es": {"Avanzado", "Eliminar datos Docker sin uso", "Comando: [docker system prune --all --force]"},
	} {
		translator := New(locale)
		got := []string{
			translator.Text(MessageAdvancedTitle),
			translator.Text(MessageAdvancedDeleteSystem),
			translator.Text(MessageAdvancedCommand, "docker system prune --all --force"),
		}
		for index := range expected {
			if got[index] != expected[index] {
				t.Fatalf("locale %s message %d = %q, want %q", locale, index, got[index], expected[index])
			}
		}
	}
}

func TestTranslatorDecimals(t *testing.T) {
	if got := New("en").Decimal(1234.5, 2); got != "1234.50" {
		t.Fatalf("expected English decimal, got %q", got)
	}
	if got := New("es").Decimal(1234.5, 2); got != "1234,50" {
		t.Fatalf("expected Spanish decimal, got %q", got)
	}
	if got := New("es").Decimal(12.6, -1); got != "13" {
		t.Fatalf("expected negative precision to use zero, got %q", got)
	}
}

func TestTranslatorPlurals(t *testing.T) {
	tests := []struct {
		locale string
		count  int
		want   string
	}{
		{locale: "en", count: 1, want: "1 view"},
		{locale: "en", count: 2, want: "2 views"},
		{locale: "es", count: 1, want: "1 vista"},
		{locale: "es", count: 0, want: "0 vistas"},
	}
	for _, test := range tests {
		if got := New(test.locale).Plural(sharedui.MessageShellViewCount, test.count); got != test.want {
			t.Errorf("Plural(%q, %d) = %q, want %q", test.locale, test.count, got, test.want)
		}
	}
}

func TestCatalogsContainAllSharedMessages(t *testing.T) {
	if len(english) != len(spanish) {
		t.Fatalf("catalog sizes differ: English=%d Spanish=%d", len(english), len(spanish))
	}
	for id := range english {
		if _, ok := spanish[id]; !ok {
			t.Errorf("Spanish catalog is missing %q", id)
		}
	}
	for id := range spanish {
		if _, ok := english[id]; !ok {
			t.Errorf("English catalog is missing %q", id)
		}
	}
}

func TestDtopCatalogFallbackAndPluralZeroOneMany(t *testing.T) {
	translator := &Translator{locale: "es", messages: map[string]message{
		MessageTabContainers: spanish[MessageTabContainers],
	}}
	if got := translator.Text(MessageTabContainers); got != "Contenedores" {
		t.Fatalf("Spanish catalog message = %q", got)
	}
	if got := translator.Text(MessageActionDelete); got != "Delete" {
		t.Fatalf("missing Spanish dtop message did not fall back to English: %q", got)
	}
	for count, want := range map[int]string{0: "0 contenedores", 1: "1 contenedor", 4: "4 contenedores"} {
		if got := New("es").Plural(MessageUsageContainers, count); got != want {
			t.Errorf("Spanish plural %d = %q, want %q", count, got, want)
		}
	}
}
