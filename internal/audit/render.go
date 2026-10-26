package audit

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiDim    = "\x1b[2m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiCyan   = "\x1b[36m"
)

// Render writes the human-readable report. Colour is optional so the same
// function serves terminals, redirected files and CI logs.
func Render(w io.Writer, rep Report, color bool) {
	p := painter{on: color}

	fmt.Fprintf(w, "%s %s\n", p.bold("envdoctor"), rep.Version)
	fmt.Fprintf(w, "%s\n\n", p.dim(rep.OS+" · "+rep.Arch))

	score := fmt.Sprintf("%d / 100", rep.Score)
	fmt.Fprintf(w, "  %s  %s\n", p.score(rep.Score, score), p.score(rep.Score, "grade "+rep.Grade))
	fmt.Fprintf(w, "  %s\n\n", p.dim(verdict(rep.Grade)))

	sorted := append([]Finding(nil), rep.Findings...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Severity.Rank() != sorted[j].Severity.Rank() {
			return sorted[i].Severity.Rank() < sorted[j].Severity.Rank()
		}
		return sorted[i].ID < sorted[j].ID
	})

	for _, f := range sorted {
		if f.Severity == SevOK {
			continue
		}
		fmt.Fprintf(w, "  %s %s\n", p.symbol(f.Severity), p.bold(f.Title))
		for _, line := range wrap(f.Detail, 96) {
			fmt.Fprintf(w, "      %s\n", line)
		}
		if f.Fix != "" {
			for i, line := range wrap(f.Fix, 92) {
				if i == 0 {
					fmt.Fprintf(w, "      %s %s\n", p.dim("fix:"), line)
					continue
				}
				fmt.Fprintf(w, "           %s\n", line)
			}
		}
		fmt.Fprintln(w)
	}

	footer := fmt.Sprintf("%s of %s clean", formatInt(rep.Clean), plural(rep.Checks, "check"))
	if rep.Clean == rep.Checks {
		footer = fmt.Sprintf("all %s clean", plural(rep.Checks, "check"))
	}
	if rep.FreeBytes > 0 {
		footer += fmt.Sprintf(" · %s reclaimable", humanBytes(rep.FreeBytes))
	}
	fmt.Fprintf(w, "  %s\n", p.dim(footer))
}

type painter struct{ on bool }

func (p painter) paint(code, s string) string {
	if !p.on {
		return s
	}
	return code + s + ansiReset
}

func (p painter) bold(s string) string  { return p.paint(ansiBold, s) }
func (p painter) dim(s string) string   { return p.paint(ansiDim, s) }
func (p painter) red(s string) string   { return p.paint(ansiRed, s) }
func (p painter) green(s string) string { return p.paint(ansiGreen, s) }
func (p painter) cyan(s string) string  { return p.paint(ansiCyan, s) }

func (p painter) score(n int, s string) string {
	switch {
	case n >= 90:
		return p.green(s)
	case n >= 70:
		return p.cyan(s)
	case n >= 60:
		return p.cyan(s)
	default:
		return p.red(s)
	}
}

func (p painter) symbol(s Severity) string {
	switch s {
	case SevBad:
		return p.red("x")
	case SevWarn:
		return p.red("!")
	case SevInfo:
		return p.cyan("-")
	default:
		return p.dim("+")
	}
}

// verdict gives the grade a plain-English reading so the number is actionable.
func verdict(grade string) string {
	switch grade {
	case "A":
		return "Clean. Nothing worth changing."
	case "B":
		return "Mostly clean; a few loose ends."
	case "C":
		return "Workable, but this box will bite you eventually."
	case "D":
		return "Messy. Several of these cause real failures."
	default:
		return "This setup is actively costing you time."
	}
}

// wrap breaks text on spaces so long detail lines stay readable in an 80-100
// column terminal. Words longer than the limit are left intact.
func wrap(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}
	words := strings.Fields(text)
	var lines []string
	cur := ""
	for _, w := range words {
		if cur == "" {
			cur = w
			continue
		}
		if len(cur)+1+len(w) > width {
			lines = append(lines, cur)
			cur = w
			continue
		}
		cur += " " + w
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}
