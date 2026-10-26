package audit

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// cacheTarget is one well-known package-manager cache location.
type cacheTarget struct {
	Name string
	Path string
	Size int64
}

// cacheResult carries the findings plus the total reclaimable size.
type cacheResult struct {
	findings    []Finding
	reclaimable int64
}

// cacheTargets builds the candidate list from the current environment.
func cacheTargets() []cacheTarget {
	local := os.Getenv("LOCALAPPDATA")
	roaming := os.Getenv("APPDATA")
	home, _ := os.UserHomeDir()

	candidates := []cacheTarget{
		{Name: "npm", Path: filepath.Join(local, "npm-cache")},
		{Name: "pip", Path: filepath.Join(local, "pip", "Cache")},
		{Name: "uv", Path: filepath.Join(local, "uv", "cache")},
		{Name: "go", Path: filepath.Join(local, "go-build")},
		{Name: "nuget", Path: filepath.Join(home, ".nuget", "packages")},
		{Name: "cargo", Path: filepath.Join(home, ".cargo", "registry")},
		{Name: "gradle", Path: filepath.Join(home, ".gradle", "caches")},
		{Name: "maven", Path: filepath.Join(home, ".m2", "repository")},
		{Name: "vscode", Path: filepath.Join(roaming, "Code", "Cache")},
	}

	out := make([]cacheTarget, 0, len(candidates))
	for _, c := range candidates {
		if c.Path == "" {
			continue
		}
		if fi, err := os.Stat(c.Path); err == nil && fi.IsDir() {
			out = append(out, c)
		}
	}
	return out
}

// dirSize sums regular files, ignoring anything unreadable. Symlinks are not
// followed so a junction cannot balloon the total or loop.
func dirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if info, ierr := d.Info(); ierr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func checkCaches(fast bool) cacheResult {
	if fast {
		return cacheResult{findings: []Finding{{
			ID:       "caches.skipped",
			Title:    "Cache scan skipped",
			Severity: SevInfo,
			Detail:   "Run without --fast to measure reclaimable package-manager cache.",
		}}}
	}

	targets := cacheTargets()
	if len(targets) == 0 {
		return cacheResult{findings: []Finding{{
			ID:       "caches.none",
			Title:    "No package-manager caches found",
			Severity: SevOK,
			Detail:   "Nothing to reclaim in the usual locations.",
		}}}
	}

	for i := range targets {
		targets[i].Size = dirSize(targets[i].Path)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].Size > targets[j].Size })

	var total int64
	for _, t := range targets {
		total += t.Size
	}

	top := targets
	if len(top) > 4 {
		top = top[:4]
	}
	var parts []string
	for _, t := range top {
		parts = append(parts, fmt.Sprintf("%s %s", t.Name, humanBytes(t.Size)))
	}

	sev, weight := SevOK, 0
	switch {
	case total > 20<<30:
		sev, weight = SevWarn, 6
	case total > 5<<30:
		sev, weight = SevInfo, 2
	}

	f := Finding{
		ID:       "caches.size",
		Title:    fmt.Sprintf("%s of package caches", humanBytes(total)),
		Severity: sev,
		Detail:   fmt.Sprintf("%s across %s. Top: %s.", humanBytes(total), plural(len(targets), "location"), joinComma(parts)),
		Weight:   weight,
	}
	if total > 5<<30 {
		f.Fix = "Reclaim with `npm cache clean --force`, `pip cache purge`, `uv cache prune`, or `go clean -cache`."
	}

	return cacheResult{findings: []Finding{f}, reclaimable: total}
}

// humanBytes renders a byte count with one decimal for the units people care about.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func joinComma(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}
