package audit

import (
	"os"
	"strings"
)

// Redact replaces personal and machine-identifying paths with environment
// variable names so a report can be pasted into an issue or a chat safely.
// The substitution is exact-prefix based on the current process environment,
// which is exactly the text the checks produced.
func Redact(rep *Report) {
	home, _ := os.UserHomeDir()
	replacements := []struct{ from, to string }{
		{os.Getenv("LOCALAPPDATA"), `%LOCALAPPDATA%`},
		{os.Getenv("APPDATA"), `%APPDATA%`},
		{os.Getenv("USERPROFILE"), `%USERPROFILE%`},
		{home, `%USERPROFILE%`},
		{os.Getenv("ProgramFiles"), `%ProgramFiles%`},
		{os.Getenv("ProgramFiles(x86)"), `%ProgramFiles(x86)%`},
	}

	for i := range rep.Findings {
		for _, r := range replacements {
			if r.from == "" || r.from == `\` {
				continue
			}
			rep.Findings[i].Detail = replaceFold(rep.Findings[i].Detail, r.from, r.to)
			rep.Findings[i].Fix = replaceFold(rep.Findings[i].Fix, r.from, r.to)
			rep.Findings[i].Title = replaceFold(rep.Findings[i].Title, r.from, r.to)
		}
	}
}

// replaceFold is a case-insensitive substring replacement. Windows paths are
// case-insensitive, but the environment values and the paths the filesystem
// reports can differ in casing.
func replaceFold(s, old, new string) string {
	if old == "" || len(s) < len(old) {
		return s
	}
	lowerS := strings.ToLower(s)
	lowerOld := strings.ToLower(old)
	var b strings.Builder
	i := 0
	for {
		j := strings.Index(lowerS[i:], lowerOld)
		if j < 0 {
			b.WriteString(s[i:])
			return b.String()
		}
		b.WriteString(s[i : i+j])
		b.WriteString(new)
		i += j + len(old)
		if i >= len(s) {
			return b.String()
		}
	}
}
