// Command envdoctor audits a developer machine and scores its setup.
//
// It is read-only: nothing is created, changed or removed. Use --json for
// machine-readable output and --fail-under to gate CI.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/sinnerk/envdoctor/internal/audit"
)

func main() {
	fs := flag.NewFlagSet("envdoctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		asJSON      = fs.Bool("json", false, "print the report as JSON")
		noColor     = fs.Bool("no-color", false, "disable ANSI colour")
		fast        = fs.Bool("fast", false, "skip the package-cache size scan")
		redact      = fs.Bool("redact", false, "replace home and program paths with %VARIABLE% names before printing")
		failUnder   = fs.Int("fail-under", 0, "exit 1 when the score is below this value (0 disables)")
		showVersion = fs.Bool("version", false, "print the version and exit")
	)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "envdoctor %s - audit a developer machine and score its setup\n\n", audit.Version)
		fmt.Fprintf(os.Stderr, "Usage:\n  envdoctor [flags]\n\nFlags:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nNothing is modified: every check is read-only and needs no admin rights.\n")
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	if *showVersion {
		fmt.Println("envdoctor " + audit.Version)
		return
	}

	report := audit.Run(audit.Options{Fast: *fast})
	if *redact {
		audit.Redact(&report)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintln(os.Stderr, "envdoctor:", err)
			os.Exit(1)
		}
	} else {
		color := !*noColor && os.Getenv("NO_COLOR") == "" && isTerminal(os.Stdout)
		audit.Render(os.Stdout, report, color)
	}

	if *failUnder > 0 && report.Score < *failUnder {
		os.Exit(1)
	}
}

// isTerminal avoids emitting ANSI escapes into pipes and files.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
