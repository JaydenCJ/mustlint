// Output for the `mustlint stats` subcommand: a fixed-width text table and
// a JSON envelope with the full per-keyword map.
package render

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/JaydenCJ/mustlint/internal/spec"
	"github.com/JaydenCJ/mustlint/internal/version"
)

// statsColumns are the table columns; less common keywords fold into OTHER
// to keep the table readable (JSON always carries the full map).
var statsColumns = []string{"MUST", "MUST NOT", "SHOULD", "SHOULD NOT", "MAY"}

// WriteStatsText renders the inventory table with a totals row.
func WriteStatsText(w io.Writer, all []spec.Stats) {
	nameW := len("total")
	for _, st := range all {
		if len(st.File) > nameW {
			nameW = len(st.File)
		}
	}
	fmt.Fprintf(w, "%-*s  %6s  %8s  %6s  %10s  %5s  %5s  %5s  %4s\n",
		nameW, "file", "MUST", "MUST NOT", "SHOULD", "SHOULD NOT", "MAY", "other", "reqs", "ids")

	total := spec.Stats{File: "total", Keywords: map[string]int{}}
	for _, st := range all {
		writeStatsRow(w, nameW, st)
		for k, v := range st.Keywords {
			total.Keywords[k] += v
		}
		total.Normative += st.Normative
		total.Defined += st.Defined
	}
	if len(all) != 1 {
		writeStatsRow(w, nameW, total)
	}
}

func writeStatsRow(w io.Writer, nameW int, st spec.Stats) {
	inColumns := map[string]bool{}
	shown := make([]int, len(statsColumns))
	for i, k := range statsColumns {
		inColumns[k] = true
		shown[i] = st.Keywords[k]
	}
	other := 0
	for k, v := range st.Keywords {
		if !inColumns[k] {
			other += v
		}
	}
	fmt.Fprintf(w, "%-*s  %6d  %8d  %6d  %10d  %5d  %5d  %5d  %4d\n",
		nameW, st.File, shown[0], shown[1], shown[2], shown[3], shown[4],
		other, st.Normative, st.Defined)
}

// jsonStats is the stats-command envelope.
type jsonStats struct {
	Tool          string          `json:"tool"`
	Version       string          `json:"version"`
	SchemaVersion int             `json:"schema_version"`
	Files         []jsonFileStats `json:"files"`
}

type jsonFileStats struct {
	File      string         `json:"file"`
	Keywords  map[string]int `json:"keywords"` // JSON keys marshal sorted
	Normative int            `json:"normative_sentences"`
	Defined   int            `json:"requirement_ids"`
}

// WriteStatsJSON renders the inventory as JSON.
func WriteStatsJSON(w io.Writer, all []spec.Stats) error {
	rep := jsonStats{
		Tool: "mustlint", Version: version.Version, SchemaVersion: 1,
		Files: make([]jsonFileStats, 0, len(all)),
	}
	for _, st := range all {
		rep.Files = append(rep.Files, jsonFileStats{
			File: st.File, Keywords: st.Keywords,
			Normative: st.Normative, Defined: st.Defined,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}
