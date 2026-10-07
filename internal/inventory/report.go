// Package inventory records crypto evidence without making release-gating decisions.
package inventory

import (
	"encoding/json"
	"fmt"
	"io"
	"path"
	"sort"
)

const SchemaVersion = "crypto-inventory/v1"

// Report is shared by source, binary, and image inventory commands.
type Report struct {
	SchemaVersion  string     `json:"schemaVersion"`
	PolicyVersion  string     `json:"policyVersion"`
	ScannerVersion string     `json:"scannerVersion"`
	Component      string     `json:"component,omitempty"`
	Mode           string     `json:"mode"`
	Artifacts      []Artifact `json:"artifacts"`
	Limitations    []string   `json:"limitations"`
}

// Artifact preserves coverage and provenance even when analysis fails.
type Artifact struct {
	Path          string            `json:"path"`
	Component     string            `json:"component,omitempty"`
	Image         string            `json:"image,omitempty"`
	SHA256        string            `json:"sha256,omitempty"`
	SourceCommit  string            `json:"sourceCommit,omitempty"`
	GoVersion     string            `json:"goVersion,omitempty"`
	BuildSettings map[string]string `json:"buildSettings,omitempty"`
	Coverage      string            `json:"coverage"`
	Errors        []string          `json:"errors,omitempty"`
	Findings      []Finding         `json:"findings"`
}

// Finding describes a candidate classification, not a confirmed violation.
type Finding struct {
	Package        string   `json:"package"`
	Origin         string   `json:"origin"`
	Symbol         string   `json:"symbol"`
	ModuleVersion  string   `json:"moduleVersion,omitempty"`
	Classification string   `json:"classification"`
	Candidates     []string `json:"classificationCandidates,omitempty"`
	Reason         string   `json:"reason"`
	Evidence       string   `json:"evidence"`
	Source         string   `json:"source,omitempty"`
	CallChain      []string `json:"callChain,omitempty"`
	Guard          string   `json:"guard"`
	PQCBoundary    string   `json:"pqcBoundary"`
}

func NewReport(mode, component string) *Report {
	return &Report{
		SchemaVersion: SchemaVersion,
		PolicyVersion: defaultPolicy.Version,
		Mode:          mode, Component: component, Artifacts: []Artifact{},
		Limitations: []string{
			"Classifications are candidates based on the Jira taxonomy; the authoritative policy document has not been validated.",
			"Evidence does not prove execution in FIPS mode, security intent, runtime guard correctness, or cluster-boundary negotiation.",
			"No exceptions are granted and crypto findings do not block releases in inventory mode.",
		},
	}
}

func (r *Report) Incomplete() bool {
	for _, a := range r.Artifacts {
		if a.Coverage != "analyzed" {
			return true
		}
	}
	return len(r.Artifacts) == 0
}

// RequirePaths records missing expected image binaries as coverage failures.
func RequirePaths(artifacts []Artifact, expected []string, image string) []Artifact {
	present := make(map[string]bool)
	for _, artifact := range artifacts {
		present[artifact.Path] = true
	}
	for _, name := range expected {
		name = path.Clean("/" + name)
		if present[name] {
			continue
		}
		artifacts = append(artifacts, Artifact{Path: name, Image: image, Coverage: "failed", Errors: []string{"expected Go executable missing from inventory"}, Findings: []Finding{}})
		present[name] = true
	}
	return artifacts
}

func (r *Report) Write(w io.Writer, format string) error {
	sort.Slice(r.Artifacts, func(i, j int) bool {
		if r.Artifacts[i].Image != r.Artifacts[j].Image {
			return r.Artifacts[i].Image < r.Artifacts[j].Image
		}
		return r.Artifacts[i].Path < r.Artifacts[j].Path
	})
	for i := range r.Artifacts {
		findings := r.Artifacts[i].Findings
		sort.Slice(findings, func(i, j int) bool { return findings[i].Symbol < findings[j].Symbol })
	}
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	if format != "text" {
		return fmt.Errorf("unsupported inventory format %q", format)
	}
	if _, err := fmt.Fprintf(w, "Crypto inventory (%s, %s)\n", r.SchemaVersion, r.PolicyVersion); err != nil {
		return err
	}
	for _, a := range r.Artifacts {
		if _, err := fmt.Fprintf(w, "\n%s [%s] image=%s\n", a.Path, a.Coverage, a.Image); err != nil {
			return err
		}
		for _, e := range a.Errors {
			if _, err := fmt.Fprintf(w, "  Coverage error: %s\n", e); err != nil {
				return err
			}
		}
		for _, f := range a.Findings {
			if _, err := fmt.Fprintf(w, "  %s %s [%s; guard=%s; P1=%s]\n    %s\n", f.Classification, f.Symbol, f.Evidence, f.Guard, f.PQCBoundary, f.Reason); err != nil {
				return err
			}
		}
	}
	for _, l := range r.Limitations {
		if _, err := fmt.Fprintf(w, "\nLimit: %s\n", l); err != nil {
			return err
		}
	}
	return nil
}
