package provider

import (
	"errors"
	"strings"
	"testing"
	"unicode"
)

func TestErrorDetailStripsControlCharacters(t *testing.T) {
	got := errorDetail(errors.New("\x1b[31mforged\x07\r\nline\x00"))

	for _, r := range got {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			t.Fatalf("errorDetail left control character %U in %q", r, got)
		}
	}
	if !strings.Contains(got, "forged\nline") {
		t.Fatalf("errorDetail lost the readable text: %q", got)
	}
}

func TestErrorDetailTruncates(t *testing.T) {
	got := errorDetail(errors.New(strings.Repeat("x", 4*maxErrorDetail)))
	if !strings.HasSuffix(got, "… (truncated)") {
		t.Fatalf("long error was not truncated: %q", got)
	}
	if len([]rune(got)) > maxErrorDetail+len([]rune("… (truncated)")) {
		t.Fatalf("truncated error is still %d runes", len([]rune(got)))
	}
}

func TestErrorDetailNil(t *testing.T) {
	if got := errorDetail(nil); got != "" {
		t.Fatalf("errorDetail(nil) = %q, want empty", got)
	}
}
