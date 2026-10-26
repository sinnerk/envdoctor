package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormaliseDir(t *testing.T) {
	cases := []struct{ in, want string }{
		{`C:\Tools\Bin`, `c:/tools/bin`},
		{`C:\Tools\Bin\`, `c:/tools/bin`},
		{`C:\Tools\Bin/`, `c:/tools/bin`},
		{`  C:\Tools\Bin  `, `c:/tools/bin`},
		{`/usr/local/bin`, `/usr/local/bin`},
	}
	for _, c := range cases {
		if got := normaliseDir(c.in); got != c.want {
			t.Errorf("normaliseDir(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatInt(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{2047, "2,047"},
		{1234567, "1,234,567"},
	}
	for _, c := range cases {
		if got := formatInt(c.in); got != c.want {
			t.Errorf("formatInt(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestScore(t *testing.T) {
	if s, g := score(nil); s != 100 || g != "A" {
		t.Errorf("clean machine = %d/%s, want 100/A", s, g)
	}

	// A single finding cannot deduct more than 20 even if it asks for more.
	if s, _ := score([]Finding{{Weight: 90}}); s != 80 {
		t.Errorf("weight cap not applied: score = %d, want 80", s)
	}

	// The score floors at zero rather than going negative.
	if s, g := score([]Finding{{Weight: 20}, {Weight: 20}, {Weight: 20}, {Weight: 20}, {Weight: 20}}); s != 0 || g != "F" {
		t.Errorf("floor = %d/%s, want 0/F", s, g)
	}
}

func TestGradeBoundaries(t *testing.T) {
	// Each finding deducts 10, so five of them reach the 100-point cap.
	cases := []struct {
		findings int
		want     string
	}{
		{0, "A"},
		{1, "A"},
		{2, "B"},
		{3, "C"},
		{4, "D"},
		{5, "F"},
	}
	for _, c := range cases {
		f := make([]Finding, c.findings)
		for i := range f {
			f[i].Weight = 10
		}
		if _, g := score(f); g != c.want {
			t.Errorf("%d findings graded %s, want %s", c.findings, g, c.want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{5 << 30, "5.0 GB"},
	}
	for _, c := range cases {
		if got := humanBytes(c.in); got != c.want {
			t.Errorf("humanBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsStoreStub(t *testing.T) {
	if !isStoreStub(`C:\Users\x\AppData\Local\Microsoft\WindowsApps\python.exe`, 0) {
		t.Error("zero-byte WindowsApps alias not detected")
	}
	if isStoreStub(`C:\Users\x\AppData\Local\Microsoft\WindowsApps\python.exe`, 42) {
		t.Error("non-empty WindowsApps file reported as a stub")
	}
	if isStoreStub(`C:\Python312\python.exe`, 0) {
		t.Error("empty file outside WindowsApps reported as a stub")
	}
}

func TestFindInstallsOrderAndDedup(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	dir3 := t.TempDir()

	write := func(dir, name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(dir1, "node.exe")
	write(dir1, "node.cmd") // same directory: must count once
	write(dir2, "node.exe")
	// dir3 intentionally empty

	hits := findInstalls("node", []string{dir1, dir2, dir3}, []string{"", ".exe", ".cmd"})
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2: %+v", len(hits), hits)
	}
	if hits[0].Path != filepath.Join(dir1, "node.exe") {
		t.Errorf("first hit = %q, want %q", hits[0].Path, filepath.Join(dir1, "node.exe"))
	}
	if hits[1].Path != filepath.Join(dir2, "node.exe") {
		t.Errorf("second hit = %q, want %q", hits[1].Path, filepath.Join(dir2, "node.exe"))
	}
}

func TestInstallRootCollapsesBinAndCmd(t *testing.T) {
	got := installRoot(`C:\tools\git\cmd`)
	want := normaliseDir(`C:\tools\git`)
	if got != want {
		t.Errorf("installRoot = %q, want %q", got, want)
	}
	if got := installRoot(`C:\Program Files\nodejs`); got != normaliseDir(`C:\Program Files\nodejs`) {
		t.Errorf("plain install dir rewritten: %q", got)
	}
	if got := installRoot("/usr/local/bin"); got != "/usr/local" {
		t.Errorf("posix install root = %q, want /usr/local", got)
	}
}

func TestFamilyOf(t *testing.T) {
	pairs := map[string]string{
		"python3": "Python",
		"pip":     "Python",
		"npm":     "Node",
		"javac":   "Java",
		"rustc":   "Rust",
		"git":     "Git",
		"weird":   "weird",
	}
	for in, want := range pairs {
		if got := familyOf(in); got != want {
			t.Errorf("familyOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMaskEmail(t *testing.T) {
	cases := map[string]string{
		"a@b.com":           "**@b.com",
		"alice@example.com": "a****@example.com",
		"notanemail":        "***",
	}
	for in, want := range cases {
		if got := maskEmail(in); got != want {
			t.Errorf("maskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWrap(t *testing.T) {
	long := strings.Repeat("word ", 40)
	lines := wrap(long, 40)
	if len(lines) < 2 {
		t.Fatalf("expected wrapping, got %d line(s)", len(lines))
	}
	for _, l := range lines {
		if len(l) > 40 {
			t.Errorf("line %q exceeds width", l)
		}
	}
	if short := wrap("tiny", 40); len(short) != 1 || short[0] != "tiny" {
		t.Errorf("short text altered: %v", short)
	}
}

func TestCategory(t *testing.T) {
	cases := map[string]string{
		"path.dead":      "path",
		"tools.multiple": "tools",
		"nodot":          "nodot",
	}
	for in, want := range cases {
		if got := category(in); got != want {
			t.Errorf("category(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPluralOf(t *testing.T) {
	if got := pluralOf(1, "entry", "entries"); got != "1 entry" {
		t.Errorf("singular = %q", got)
	}
	if got := pluralOf(2, "entry", "entries"); got != "2 entries" {
		t.Errorf("plural = %q", got)
	}
}

func TestIsLauncher(t *testing.T) {
	// Shims that live inside another install must not create phantom duplicates.
	for _, name := range []string{"pip", "pip3", "py", "npm", "npx", "pnpm", "yarn", "bun"} {
		if !isLauncher(name) {
			t.Errorf("%s should be a launcher", name)
		}
	}
	for _, name := range []string{"python", "python3", "node", "git", "java"} {
		if isLauncher(name) {
			t.Errorf("%s should not be a launcher", name)
		}
	}
}

func TestDisplayPathsResolvesCollisions(t *testing.T) {
	got := displayPaths([]string{`C:\Program Files\Go\bin`, `C:\tools\go\x64\bin`}, 4)
	if len(got) != 2 {
		t.Fatalf("got %v", got)
	}
	if got[0] == got[1] {
		t.Fatalf("two different installs rendered identically: %v", got)
	}
	if !strings.Contains(got[0], "x64") || !strings.Contains(got[1], "x64") {
		t.Logf("rendered as %v", got)
	}
}

func TestDeadEntriesDeduped(t *testing.T) {
	t.Setenv("PATH", `C:\definitely-missing-a;C:\definitely-missing-a;C:\definitely-missing-b`)

	findings := checkPath()
	var dead *Finding
	for i := range findings {
		if findings[i].ID == "path.dead" {
			dead = &findings[i]
			break
		}
	}
	if dead == nil {
		t.Fatal("expected a path.dead finding")
	}
	if dead.Weight != 4 {
		t.Errorf("weight = %d, want 4 (two unique entries)", dead.Weight)
	}
	if strings.Count(dead.Detail, "definitely-missing-a") != 1 {
		t.Errorf("duplicate dead entry listed more than once: %s", dead.Detail)
	}
}

func TestCheckGitSkipsOnCI(t *testing.T) {
	t.Setenv("CI", "true")
	findings := checkGit()
	if len(findings) != 1 {
		t.Fatalf("expected one informational finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Severity != SevOK || findings[0].Weight != 0 {
		t.Errorf("CI finding should be free: %+v", findings[0])
	}
}
