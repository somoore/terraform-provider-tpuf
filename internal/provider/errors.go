package provider

import (
	"errors"
	"net/http"
	"strings"
	"unicode"

	tpuf "github.com/turbopuffer/turbopuffer-go"
)

func isNotFoundErr(err error) bool {
	var apiErr *tpuf.Error
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// maxErrorDetail bounds how much untrusted API text reaches a diagnostic.
const maxErrorDetail = 1024

// errorDetail renders an error for a user-facing diagnostic. Control characters
// (including ANSI escapes) are stripped and the text is truncated, so a hostile
// or compromised API endpoint cannot forge terminal or CI log lines. Newlines
// and tabs are kept because they make multi-line API errors readable.
func errorDetail(err error) string {
	if err == nil {
		return ""
	}
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, err.Error())
	runes := []rune(cleaned)
	if len(runes) > maxErrorDetail {
		return string(runes[:maxErrorDetail]) + "… (truncated)"
	}
	return cleaned
}
