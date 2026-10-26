// Package audit contains the read-only environment checks that envdoctor runs.
//
// Every check is side-effect free: nothing is written, no service is touched and
// no administrator rights are required. Findings carry a deduction weight so the
// final score is reproducible and explainable rather than a black box.
package audit

import (
	"fmt"
	"strings"
	"time"
)

// Version is stamped into reports and compared by the update hint.
var Version = "0.1.0"

// Severity describes how much a finding matters.
type Severity string

const (
	SevOK   Severity = "ok"
	SevInfo Severity = "info"
	SevWarn Severity = "warn"
	SevBad  Severity = "bad"
)

// Rank orders severities for sorting, worst first.
func (s Severity) Rank() int {
	switch s {
	case SevBad:
		return 0
	case SevWarn:
		return 1
	case SevInfo:
		return 2
	default:
		return 3
	}
}

// Finding is a single observation about the machine.
type Finding struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Severity Severity `json:"severity"`
	Detail   string   `json:"detail"`
	Fix      string   `json:"fix,omitempty"`
	Weight   int      `json:"weight"`
}

// Report is the full result of one run.
type Report struct {
	Tool      string    `json:"tool"`
	Version   string    `json:"version"`
	Generated time.Time `json:"generated"`
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	Findings  []Finding `json:"findings"`
	Score     int       `json:"score"`
	Grade     string    `json:"grade"`
	Checks    int       `json:"checks"`
	Problems  int       `json:"problems"`
	Clean     int       `json:"clean"`
	FreeBytes int64     `json:"reclaimable_bytes"`
}

// run stores accumulated findings while checks execute.
type run struct {
	findings []Finding
}

func (r *run) add(findings ...Finding) {
	for _, f := range findings {
		if f.Severity == "" {
			f.Severity = SevInfo
		}
		r.findings = append(r.findings, f)
	}
}

// Options controls which checks execute.
type Options struct {
	// Fast skips the slow cache-size walk.
	Fast bool
	// Tools overrides the list of executables probed for duplicate installs.
	Tools []string
	// Ports overrides the local ports probed for listeners.
	Ports []int
}

// Run executes every check and returns a scored report.
func Run(opts Options) Report {
	sys := DetectSystem()

	r := &run{}
	r.add(checkPath()...)
	r.add(checkTools(opts.Tools)...)
	r.add(checkSystem(sys)...)
	r.add(checkGit()...)
	r.add(checkPorts(opts.Ports)...)
	cache := checkCaches(opts.Fast)
	r.add(cache.findings...)

	rep := Report{
		Tool:      "envdoctor",
		Version:   Version,
		Generated: time.Now(),
		OS:        sys.Describe(),
		Arch:      platformArch(),
		Findings:  r.findings,
		Checks:    len(r.findings),
		FreeBytes: cache.reclaimable,
	}
	rep.Score, rep.Grade = score(r.findings)

	// Count categories rather than individual findings: five duplicate tools are
	// one check that failed, not five, and the footer should say so.
	cats := map[string]bool{}
	failed := map[string]bool{}
	for _, f := range r.findings {
		c := category(f.ID)
		cats[c] = true
		if f.Severity != SevOK {
			failed[c] = true
		}
	}
	rep.Checks = len(cats)
	rep.Problems = len(failed)
	rep.Clean = len(cats) - len(failed)
	return rep
}

// category is the part of a finding ID before the first dot.
func category(id string) string {
	if i := strings.Index(id, "."); i > 0 {
		return id[:i]
	}
	return id
}

// score converts deduction weights into a 0-100 score and a letter grade.
// Weights are capped so no single category can zero the result on its own.
func score(findings []Finding) (int, string) {
	total := 0
	for _, f := range findings {
		w := f.Weight
		if w < 0 {
			w = 0
		}
		if w > 20 {
			w = 20
		}
		total += w
	}
	if total > 100 {
		total = 100
	}
	s := 100 - total
	switch {
	case s >= 90:
		return s, "A"
	case s >= 80:
		return s, "B"
	case s >= 70:
		return s, "C"
	case s >= 60:
		return s, "D"
	default:
		return s, "F"
	}
}

// Describe renders the operating system for display.
func (s System) Describe() string {
	name := s.Name
	if name == "" {
		name = platformOS()
	}
	if s.Version != "" {
		name += " " + s.Version
	}
	if s.Build != "" {
		return fmt.Sprintf("%s (build %s)", name, s.Build)
	}
	return name
}
