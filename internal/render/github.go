// GitHub workflow-command output: one ::error / ::warning / ::notice line
// per finding, so findings surface as inline annotations on pull requests
// without any plugin.
package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/JaydenCJ/mustlint/internal/rules"
)

// WriteGitHub renders findings as GitHub Actions workflow commands.
func WriteGitHub(w io.Writer, fs []rules.Finding) {
	for _, f := range fs {
		level := "notice"
		switch f.Severity {
		case rules.Error:
			level = "error"
		case rules.Warning:
			level = "warning"
		}
		fmt.Fprintf(w, "::%s file=%s,line=%d,col=%d,title=mustlint %s::%s\n",
			level, escapeGitHubProp(f.File), f.Line, f.Col, f.Rule, escapeGitHub(f.Message))
	}
}

// escapeGitHub applies the workflow-command data escaping rules.
func escapeGitHub(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

// escapeGitHubProp escapes a workflow-command property value, which
// additionally reserves ":" and "," (a comma in a file path would otherwise
// truncate the property list).
func escapeGitHubProp(s string) string {
	s = escapeGitHub(s)
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}
