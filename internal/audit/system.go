package audit

import "fmt"

// System is the host description collected by the platform-specific detector.
type System struct {
	Name           string
	Version        string
	Build          string
	LongPaths      bool
	LongPathsKnown bool
}

// checkSystem reports the host and the OS-level switches that break long paths.
func checkSystem(s System) []Finding {
	name := s.Name
	if name == "" {
		name = platformOS()
	}
	if s.Version != "" {
		name += " " + s.Version
	}

	detail := platformArch()
	if s.Build != "" {
		detail = fmt.Sprintf("build %s, %s", s.Build, platformArch())
	}
	findings := []Finding{{
		ID:       "system.os",
		Title:    name,
		Severity: SevOK,
		Detail:   detail,
	}}

	if !s.LongPathsKnown {
		return findings
	}
	if s.LongPaths {
		return append(findings, Finding{
			ID:       "system.longpaths",
			Title:    "Long path support is on",
			Severity: SevOK,
			Detail:   "Paths longer than 260 characters are allowed.",
		})
	}

	return append(findings, Finding{
		ID:       "system.longpaths",
		Title:    "Long path support is off",
		Severity: SevWarn,
		Detail:   "LongPathsEnabled=0. Checkouts and builds fail once a nested path passes 260 characters, which deep node_modules and nested build folders reach easily.",
		Fix:      `Run as admin: reg add "HKLM\SYSTEM\CurrentControlSet\Control\FileSystem" /v LongPathsEnabled /t REG_DWORD /d 1 /f, then restart.`,
		Weight:   8,
	})
}
