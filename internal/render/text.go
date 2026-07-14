// Human-readable text output: one aligned line per finding plus a summary.
package render

import (
	"fmt"
	"io"

	"github.com/JaydenCJ/mustlint/internal/rules"
)

// WriteText renders findings for a terminal. With quiet=true the summary
// line is omitted and a clean run prints nothing at all.
func WriteText(w io.Writer, fs []rules.Finding, filesChecked int, quiet bool) {
	for _, f := range fs {
		fmt.Fprintf(w, "%s:%d:%d  %-7s  %-21s  %s\n",
			f.File, f.Line, f.Col, f.Severity, f.Rule, f.Message)
	}
	if quiet {
		return
	}
	if len(fs) > 0 {
		fmt.Fprintln(w)
	}
	s := Summarize(fs)
	if s.Total == 0 {
		fmt.Fprintf(w, "%s checked: no findings\n", countNoun(filesChecked, "file"))
		return
	}
	fmt.Fprintf(w, "%s checked: %s (%s, %s, %s)\n",
		countNoun(filesChecked, "file"),
		countNoun(s.Total, "finding"),
		countNoun(s.Errors, "error"),
		countNoun(s.Warnings, "warning"),
		fmt.Sprintf("%d info", s.Infos))
}
