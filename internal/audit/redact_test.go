package audit

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	t.Setenv("USERPROFILE", `C:\Users\tester`)
	t.Setenv("LOCALAPPDATA", `C:\Users\tester\AppData\Local`)

	rep := Report{Findings: []Finding{{
		ID:     "tools.multiple",
		Detail: `node: C:\Users\tester\AppData\Local\nvm\.nodejs, C:\Users\tester\scoop\node`,
		Fix:    `check C:\Users\tester\AppData\Local`,
		Title:  `PATH under C:\Users\tester is long`,
	}}}

	Redact(&rep)

	got := rep.Findings[0]
	if strings.Contains(strings.ToLower(got.Detail), "tester") {
		t.Errorf("detail still contains the user name: %q", got.Detail)
	}
	if !strings.Contains(got.Detail, `%LOCALAPPDATA%`) {
		t.Errorf("local app data not replaced: %q", got.Detail)
	}
	if !strings.Contains(got.Detail, `%USERPROFILE%\scoop\node`) {
		t.Errorf("profile not replaced: %q", got.Detail)
	}
	if strings.Contains(got.Fix, "tester") || strings.Contains(got.Title, "tester") {
		t.Errorf("fix/title not redacted: %q / %q", got.Fix, got.Title)
	}
}

func TestReplaceFold(t *testing.T) {
	cases := []struct {
		s, old, new, want string
	}{
		{`C:\Users\Bob\bin`, `c:\users\bob`, `%USERPROFILE%`, `%USERPROFILE%\bin`},
		{"nothing here", "zzz", "x", "nothing here"},
		{"aaa", "aaa", "b", "b"},
		{"aXaXa", "a", "b", "bXbXb"},
	}
	for _, c := range cases {
		if got := replaceFold(c.s, c.old, c.new); got != c.want {
			t.Errorf("replaceFold(%q, %q, %q) = %q, want %q", c.s, c.old, c.new, got, c.want)
		}
	}
}
