// Package render turns findings and statistics into the three output
// formats: human text, stable JSON (schema_version 1), and GitHub workflow
// annotations. All output is deterministic — same input, same bytes.
package render

import (
	"fmt"

	"github.com/JaydenCJ/mustlint/internal/rules"
)

// Format names one of the supported output formats.
type Format string

const (
	Text   Format = "text"
	JSON   Format = "json"
	GitHub Format = "github"
)

// ParseFormat validates a --format value.
func ParseFormat(s string) (Format, bool) {
	switch Format(s) {
	case Text, JSON, GitHub:
		return Format(s), true
	}
	return "", false
}

// Summary tallies findings by severity.
type Summary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	Infos    int `json:"infos"`
	Total    int `json:"total"`
}

// Summarize counts findings per severity.
func Summarize(fs []rules.Finding) Summary {
	var s Summary
	for _, f := range fs {
		switch f.Severity {
		case rules.Error:
			s.Errors++
		case rules.Warning:
			s.Warnings++
		default:
			s.Infos++
		}
	}
	s.Total = len(fs)
	return s
}

// plural returns "" for 1 and "s" otherwise.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// countNoun formats "3 errors" / "1 warning".
func countNoun(n int, noun string) string {
	return fmt.Sprintf("%d %s%s", n, noun, plural(n))
}
