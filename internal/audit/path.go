package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// pathStats is the raw material for the PATH checks.
type pathStats struct {
	Raw        string
	Entries    []string
	Dead       []string
	Duplicates []string
	Relative   []string
}

func inspectPath() pathStats {
	raw := os.Getenv("PATH")
	parts := strings.Split(raw, string(os.PathListSeparator))

	inPath := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), `"`)
		if p == "" {
			continue
		}
		inPath = append(inPath, p)
	}

	counts := make(map[string]int, len(inPath))
	for _, e := range inPath {
		counts[normaliseDir(e)]++
	}

	seen := make(map[string]bool, len(inPath))
	var dups []string
	for _, e := range inPath {
		k := normaliseDir(e)
		if counts[k] > 1 && !seen[k] {
			seen[k] = true
			dups = append(dups, e)
		}
	}
	sort.Strings(dups)

	var dead []string
	seenDead := map[string]bool{}
	for _, e := range inPath {
		fi, err := os.Stat(e)
		if err != nil || !fi.IsDir() {
			k := normaliseDir(e)
			if seenDead[k] {
				continue
			}
			seenDead[k] = true
			dead = append(dead, e)
		}
	}

	var relative []string
	for _, e := range inPath {
		if !filepath.IsAbs(e) {
			relative = append(relative, e)
		}
	}

	return pathStats{Raw: raw, Entries: inPath, Dead: dead, Duplicates: dups, Relative: relative}
}

// normaliseDir produces a comparison key: separator-agnostic, case-insensitive,
// with no trailing separator. Keeping the key portable means the same logic and
// tests work on Windows, macOS and Linux.
func normaliseDir(p string) string {
	p = strings.TrimSpace(p)
	p = strings.ReplaceAll(p, `\`, `/`)
	p = strings.TrimRight(p, `/`)
	return strings.ToLower(p)
}

func checkPath() []Finding {
	s := inspectPath()
	length := len(s.Raw)
	var findings []Finding

	// Windows truncates PATH in some installers at 2047 characters and the whole
	// environment block caps at 32767, so length matters independently of entries.
	switch {
	case length > 8191:
		findings = append(findings, Finding{
			ID:       "path.length",
			Title:    fmt.Sprintf("PATH is %s characters", formatInt(length)),
			Severity: SevBad,
			Detail:   "Far past the 8,191-character limit where installers and shells start truncating entries.",
			Fix:      "Move rarely used tool directories into a shell profile or a `PATH` wrapper instead of the machine-wide variable.",
			Weight:   20,
		})
	case length > 2047:
		findings = append(findings, Finding{
			ID:       "path.length",
			Title:    fmt.Sprintf("PATH is %s characters", formatInt(length)),
			Severity: SevWarn,
			Detail:   "Past 2,047 characters, the point where some installers and legacy tools truncate the variable and silently lose entries.",
			Fix:      "Remove dead and duplicate entries, or move niche tools into a short launcher directory.",
			Weight:   8,
		})
	}

	if len(s.Dead) > 0 {
		findings = append(findings, Finding{
			ID:       "path.dead",
			Title:    pluralOf(len(s.Dead), "PATH entry", "PATH entries") + " missing from disk",
			Severity: SevWarn,
			Detail:   "Missing: " + strings.Join(displayPaths(s.Dead, 5), ", ") + ". Every command lookup walks these for nothing, and they hide which tools are really installed.",
			Fix:      "Delete them from PATH (System Properties > Environment Variables, or `setx PATH`).",
			Weight:   deadPathWeight(len(s.Dead)),
		})
	}

	if len(s.Relative) > 0 {
		findings = append(findings, Finding{
			ID:       "path.relative",
			Title:    pluralOf(len(s.Relative), "relative PATH entry", "relative PATH entries"),
			Severity: SevWarn,
			Detail:   strings.Join(s.Relative, ", ") + ". A relative entry resolves against whatever directory you are in, so a file dropped there can run instead of the intended tool.",
			Fix:      "Replace relative entries with absolute paths.",
			Weight:   6,
		})
	}

	if len(s.Duplicates) > 0 {
		findings = append(findings, Finding{
			ID:       "path.duplicates",
			Title:    pluralOf(len(s.Duplicates), "directory", "directories") + " listed twice",
			Severity: SevInfo,
			Detail:   strings.Join(displayPaths(s.Duplicates, 5), ", ") + ". Harmless but it inflates PATH length and makes tool resolution slower.",
			Fix:      "Keep a single occurrence of each directory.",
			Weight:   minInt(len(s.Duplicates), 3),
		})
	}

	if len(findings) == 0 {
		findings = append(findings, Finding{
			ID:       "path.length",
			Title:    fmt.Sprintf("PATH is healthy (%s characters)", formatInt(length)),
			Severity: SevOK,
			Detail:   fmt.Sprintf("%s entries, none missing, no duplicates.", formatInt(len(s.Entries))),
		})
	}

	return findings
}

// deadPathWeight scales with the number of dead entries but stops growing at five
// so a single stale installer cannot dominate the score.
func deadPathWeight(n int) int {
	if n > 5 {
		n = 5
	}
	return n * 2
}

func formatInt(n int) string {
	s := fmt.Sprintf("%d", n)
	if n < 1000 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

// displayPaths shortens absolute paths to their last two components, adding
// components only when two different paths would otherwise render identically.
// Without this, C:\Program Files\Go\bin and C:\tools\go\x64\bin both print as
// "...\x64\bin" and the report looks wrong.
func displayPaths(paths []string, limit int) []string {
	items := append([]string(nil), paths...)
	sort.Strings(items)
	if len(items) > limit {
		items = items[:limit]
	}

	for depth := 2; depth <= 6; depth++ {
		out := make([]string, len(items))
		counts := map[string]int{}
		collision := false
		for i, p := range items {
			out[i] = shortenDepth(p, depth)
			counts[out[i]]++
			if counts[out[i]] > 1 {
				collision = true
			}
		}
		if !collision {
			return out
		}
	}
	return items
}

// shortenDepth keeps the last n components of a path, preserving its separator
// style so Windows and POSIX paths both read naturally.
func shortenDepth(p string, depth int) string {
	parts := strings.FieldsFunc(p, func(r rune) bool { return r == '\\' || r == '/' })
	if len(parts) <= depth {
		return p
	}
	sep := `\`
	if !strings.Contains(p, `\`) {
		sep = "/"
	}
	return "..." + sep + strings.Join(parts[len(parts)-depth:], sep)
}
