package localization

import "testing"

func TestNormalize(t *testing.T) {
	tests := map[string]string{"fr-CH": "fr", "de_DE": "de", "pt-br": "pt-BR", "hi-IN": "hi", "unknown": "en"}
	for input, expected := range tests {
		if got := Normalize(input); got != expected {
			t.Fatalf("Normalize(%q) = %q, want %q", input, got, expected)
		}
	}
}
