package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultTools are the executables most commonly installed more than once on a
// Windows developer machine. Order is stable so reports diff cleanly.
var DefaultTools = []string{
	"python", "python3", "py", "pip", "pip3",
	"node", "npm", "pnpm", "yarn",
	"git", "go", "cargo", "rustc",
	"java", "javac", "dotnet", "code", "uv",
}

// toolHit is one executable found for a tool name.
type toolHit struct {
	Path string
	Size int64
	Stub bool
}

// pathDirs returns the de-duplicated PATH directories in order.
func pathDirs() []string {
	dirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	out := make([]string, 0, len(dirs))
	seen := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		d = strings.Trim(strings.TrimSpace(d), `"`)
		if d == "" {
			continue
		}
		k := normaliseDir(d)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, d)
	}
	return out
}

// executableExts returns the Windows search order for executable suffixes.
func executableExts() []string {
	exts := []string{""}
	pathext := os.Getenv("PATHEXT")
	if pathext == "" {
		pathext = ".COM;.EXE;.BAT;.CMD"
	}
	for _, e := range strings.Split(pathext, ";") {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		exts = append(exts, strings.ToLower(e))
	}
	return exts
}

// findInstalls walks PATH the way the shell does and returns every hit, in
// resolution order. Unlike exec.LookPath it does not stop at the first match.
func findInstalls(name string, dirs []string, exts []string) []toolHit {
	var hits []toolHit
	for _, dir := range dirs {
		for _, ext := range exts {
			candidate := filepath.Join(dir, name+ext)
			fi, err := os.Stat(candidate)
			if err != nil || fi.IsDir() {
				continue
			}
			hits = append(hits, toolHit{
				Path: candidate,
				Size: fi.Size(),
				Stub: isStoreStub(candidate, fi.Size()),
			})
			break // first suffix wins inside one directory, like the shell
		}
	}
	return hits
}

// isStoreStub reports whether a match is a Microsoft Store execution alias.
// Aliases live in WindowsApps and are zero-byte reparse points; running one with
// no matching Store package opens the Store instead of the tool.
func isStoreStub(path string, size int64) bool {
	return size == 0 && strings.Contains(strings.ToLower(path), `\windowsapps\`)
}

// familyOf collapses interchangeable command names so one installation is not
// reported twice under two aliases.
func familyOf(name string) string {
	switch strings.ToLower(name) {
	case "python", "python3", "py", "pip", "pip3":
		return "Python"
	case "node", "npm", "npx", "pnpm", "yarn", "bun":
		return "Node"
	case "java", "javac", "jar":
		return "Java"
	case "cargo", "rustc":
		return "Rust"
	case "code":
		return "VS Code"
	case "git":
		return "Git"
	case "go":
		return "Go"
	case "dotnet":
		return "dotnet"
	case "uv":
		return "uv"
	default:
		return name
	}
}

func checkTools(names []string) []Finding {
	if len(names) == 0 {
		names = DefaultTools
	}
	dirs := pathDirs()
	exts := executableExts()

	type family struct {
		roots []string
		names []string
		seen  map[string]bool
	}
	families := map[string]*family{}
	var order []string
	stubbed := map[string][]string{}

	for _, name := range names {
		hits := findInstalls(name, dirs, exts)
		if len(hits) == 0 {
			continue
		}
		key := familyOf(name)
		if _, ok := families[key]; !ok {
			families[key] = &family{}
			order = append(order, key)
		}
		fam := families[key]
		fam.names = append(fam.names, name)

		// A stub first on PATH is what actually breaks the command in a fresh shell.
		if hits[0].Stub {
			stubbed[key] = append(stubbed[key], name)
		}

		seen := fam.seen
		if seen == nil {
			seen = map[string]bool{}
			fam.seen = seen
		}
		for _, h := range hits {
			if h.Stub {
				continue
			}
			d := filepath.Dir(h.Path)
			root := installRoot(d)
			if seen[root] {
				continue
			}
			seen[root] = true
			fam.roots = append(fam.roots, d)
		}
	}

	var findings []Finding

	if len(stubbed) > 0 {
		var keys []string
		var parts []string
		for k, tools := range stubbed {
			keys = append(keys, k)
			sort.Strings(tools)
			suffix := ""
			if fam := families[k]; fam != nil && len(fam.roots) > 0 {
				suffix = " (real install shadowed)"
			} else {
				suffix = " (alias only)"
			}
			parts = append(parts, k+": "+strings.Join(tools, ", ")+suffix)
		}
		sort.Strings(keys)
		sort.Strings(parts)
		findings = append(findings, Finding{
			ID:       "tools.store-stub",
			Title:    pluralOf(len(stubbed), "tool", "tools") + " shadowed by Microsoft Store aliases",
			Severity: SevWarn,
			Detail:   strings.Join(parts, "; ") + ". These are zero-byte WindowsApps aliases: with no Store package installed they open the Microsoft Store instead of running.",
			Fix:      "Install the real tool (winget, scoop, uv) or remove %LOCALAPPDATA%\\Microsoft\\WindowsApps from PATH.",
			Weight:   8,
		})
	}

	var multi []string
	for _, key := range order {
		fam := families[key]
		if len(fam.roots) < 2 {
			continue
		}
		sort.Strings(fam.roots)
		multi = append(multi, fmt.Sprintf("%s: %s", key, strings.Join(shortenAll(fam.roots, 4), ", ")))
	}
	if len(multi) > 0 {
		sort.Strings(multi)
		findings = append(findings, Finding{
			ID:       "tools.multiple",
			Title:    pluralOf(len(multi), "tool", "tools") + " installed in more than one place",
			Severity: SevInfo,
			Detail:   strings.Join(multi, "; "),
			Fix:      "Keep one installation per tool and remove the others from PATH so the version you get is predictable.",
			Weight:   minInt(len(multi)*4, 12),
		})
	}

	if len(findings) == 0 {
		findings = append(findings, Finding{
			ID:       "tools.multiple",
			Title:    "No duplicate tool installs",
			Severity: SevOK,
			Detail:   fmt.Sprintf("%s probed, each resolving to one location.", formatInt(len(names))),
		})
	}

	return findings
}

// pluralOf renders "1 entry" / "3 entries" with an irregular plural.
func pluralOf(n int, singular, plural string) string {
	if n == 1 {
		return formatInt(n) + " " + singular
	}
	return formatInt(n) + " " + plural
}

// installRoot collapses the common bin/cmd/sbin layout so one installation that
// exposes two directories is not reported as two installations. Java 17 and 21
// still count as separate roots, which is the distinction that matters.
func installRoot(dir string) string {
	parts := strings.FieldsFunc(dir, func(r rune) bool { return r == '\\' || r == '/' })
	if n := len(parts); n > 1 {
		switch strings.ToLower(parts[n-1]) {
		case "bin", "cmd", "sbin", "scripts":
			root := strings.ToLower(strings.Join(parts[:n-1], "/"))
			if strings.HasPrefix(dir, "/") {
				root = "/" + root
			}
			return root
		}
	}
	return normaliseDir(dir)
}

// plural renders "1 command" / "3 commands".
func plural(n int, word string) string {
	return pluralOf(n, word, word+"s")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
