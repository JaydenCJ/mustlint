// Stable JSON output for machines. The envelope carries schema_version so
// downstream consumers can detect breaking changes.
package render

import (
	"encoding/json"
	"io"

	"github.com/JaydenCJ/mustlint/internal/rules"
	"github.com/JaydenCJ/mustlint/internal/version"
)

// jsonFinding is the wire form of one finding.
type jsonFinding struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Col      int    `json:"col"`
	Severity string `json:"severity"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

// jsonReport is the check-command envelope.
type jsonReport struct {
	Tool          string        `json:"tool"`
	Version       string        `json:"version"`
	SchemaVersion int           `json:"schema_version"`
	FilesChecked  int           `json:"files_checked"`
	Summary       Summary       `json:"summary"`
	Findings      []jsonFinding `json:"findings"`
}

// WriteJSON renders the findings envelope with two-space indentation.
func WriteJSON(w io.Writer, fs []rules.Finding, filesChecked int) error {
	rep := jsonReport{
		Tool:          "mustlint",
		Version:       version.Version,
		SchemaVersion: 1,
		FilesChecked:  filesChecked,
		Summary:       Summarize(fs),
		Findings:      make([]jsonFinding, 0, len(fs)),
	}
	for _, f := range fs {
		rep.Findings = append(rep.Findings, jsonFinding{
			File: f.File, Line: f.Line, Col: f.Col,
			Severity: f.Severity.String(), Rule: f.Rule, Message: f.Message,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
