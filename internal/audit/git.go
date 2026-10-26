package audit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// gitValue reads one global git setting. The second return reports whether git
// answered at all, which distinguishes "unset" from "git missing".
func gitValue(key string) (string, bool) {
	cmd := exec.Command("git", "config", "--global", "--get", key)
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func gitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// isCI reports whether this looks like a hosted build environment, where a
// missing global git identity is expected rather than a fault.
func isCI() bool {
	return os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != ""
}

func checkGit() []Finding {
	if isCI() {
		return []Finding{{
			ID:       "git.identity",
			Title:    "CI environment",
			Severity: SevOK,
			Detail:   "Git identity and line-ending checks are skipped here; hosted runners set identity per checkout.",
		}}
	}

	if !gitInstalled() {
		return []Finding{{
			ID:       "git.identity",
			Title:    "Git not installed",
			Severity: SevOK,
			Detail:   "Identity and line-ending checks skipped.",
		}}
	}

	var findings []Finding

	email, ok := gitValue("user.email")
	name, _ := gitValue("user.name")
	missing := []string{}
	if !ok || email == "" {
		missing = append(missing, "user.email")
	}
	if name == "" {
		missing = append(missing, "user.name")
	}

	if len(missing) > 0 {
		findings = append(findings, Finding{
			ID:       "git.identity",
			Title:    "Git identity is incomplete",
			Severity: SevWarn,
			Detail:   "Missing global setting(s): " + strings.Join(missing, ", ") + ". New commits will be rejected or attributed to a placeholder.",
			Fix:      "git config --global user.name \"Your Name\" && git config --global user.email \"you@example.com\"",
			Weight:   6,
		})
	} else {
		findings = append(findings, Finding{
			ID:       "git.identity",
			Title:    "Git identity configured",
			Severity: SevOK,
			Detail:   fmt.Sprintf("%s <%s>", name, maskEmail(email)),
		})
	}

	// core.autocrlf decides whether a Windows checkout rewrites every line ending.
	// Unset is not wrong, but it is the usual cause of whole-file diffs.
	if v, answered := gitValue("core.autocrlf"); !answered || v == "" {
		findings = append(findings, Finding{
			ID:       "git.lineendings",
			Title:    "core.autocrlf is unset",
			Severity: SevInfo,
			Detail:   "On Windows the usual choice is `true` for solo work or `input` when the repo ships .gitattributes.",
			Fix:      "git config --global core.autocrlf true",
			Weight:   1,
		})
	}

	// core.longpaths only matters when the OS-level switch is off, but setting it
	// makes git itself immune to the 260-character limit.
	if v, answered := gitValue("core.longpaths"); !answered || !strings.EqualFold(v, "true") {
		findings = append(findings, Finding{
			ID:       "git.longpaths",
			Title:    "Git long-path support is off",
			Severity: SevInfo,
			Detail:   "Long paths in deep directory trees can fail on checkout or clone.",
			Fix:      "git config --global core.longpaths true",
			Weight:   1,
		})
	}

	return findings
}

// maskEmail keeps the domain but hides the local part so reports can be shared.
func maskEmail(e string) string {
	at := strings.LastIndex(e, "@")
	if at <= 0 {
		return "***"
	}
	local := e[:at]
	if len(local) <= 2 {
		return "**" + e[at:]
	}
	return local[:1] + strings.Repeat("*", len(local)-1) + e[at:]
}
