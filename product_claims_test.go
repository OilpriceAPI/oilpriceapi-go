package oilpriceapi

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// publicSurfaceGlobs are the locations that publish to a reader outside this
// repository: markdown rendered on GitHub, and non-test Go sources whose doc
// comments pkg.go.dev serves verbatim.
var publicSurfaceGlobs = []string{"*.md", "*.go", "example/*.go"}

// publicSurfaceFiles returns every distributed file, expanded from
// publicSurfaceGlobs rather than hand-listed. The "99.9% uptime" claim that
// shipped in v1.0.0 through v1.3.1 was not missed by a bad pattern — the
// "uptime or SLA" pattern below matches it exactly. It shipped because this
// function used to return a hand-maintained list of nine files, and a
// hand-maintained list stops describing the repository the moment anyone adds
// a file to it. TestClaimGuardCoversEveryPublishedFile cross-checks this
// expansion against a recursive walk.
func publicSurfaceFiles(t *testing.T) []string {
	t.Helper()

	var files []string
	seen := map[string]bool{}
	for _, glob := range publicSurfaceGlobs {
		matches, err := filepath.Glob(glob)
		if err != nil {
			t.Fatalf("glob %q: %v", glob, err)
		}
		for _, match := range matches {
			path := filepath.ToSlash(match)
			if strings.HasSuffix(path, "_test.go") || seen[path] {
				continue
			}
			seen[path] = true
			files = append(files, path)
		}
	}
	if len(files) == 0 {
		t.Fatal("public surface expanded to zero files; the claim guard would silently pass")
	}
	sort.Strings(files)
	return files
}

// forbiddenClaims are quantified or contractual promises that must not appear
// in anything this module distributes. We sell access, coverage, format,
// latency and cost; a number we do not measure and do not contractually offer
// is not ours to print.
var forbiddenClaims = map[string]*regexp.Regexp{
	"fixed catalog total":   regexp.MustCompile(`(?i)\b\d+\+\s+(commodit|endpoint|api)`),
	"fixed update cadence":  regexp.MustCompile(`(?i)(updated|refresh(ed)?)\s+every\s+\d+|every\s+\d+\s+minutes`),
	"unreviewed plan name":  regexp.MustCompile(`(?i)professional\+|starter plan|scale tier`),
	"unreviewed plan price": regexp.MustCompile(`(?i)\$\d+(\.\d+)?\s*(/|per\s+)(mo(nth)?|year)`),
	"uptime or SLA":         regexp.MustCompile(`(?i)\b\d+(\.\d+)?%\s+uptime|\bSLA\b`),
	"price comparison":      regexp.MustCompile(`(?i)bloomberg|\d+(\.\d+)?%\s+less\s+cost`),
	"quota promise":         regexp.MustCompile(`(?i)does\s+not\s+consume.{0,40}quota|\bunlimited\b`),
	"universal catalog":     regexp.MustCompile(`(?i)\ball\s+(latest\s+)?prices\b|\ball\s+commodit`),
	"real-time claim":       regexp.MustCompile(`(?i)\breal[- ]time\b`),
	"free-tier claim":       regexp.MustCompile(`(?i)\bfree\s+tier\b`),
}

func TestPublicSurfaceClaimDrift(t *testing.T) {
	publicFiles := publicSurfaceFiles(t)
	forbidden := forbiddenClaims

	for _, path := range publicFiles {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for label, pattern := range forbidden {
			if match := pattern.Find(content); match != nil {
				t.Errorf("%s contains %s %q; link to the reviewed product contract instead", path, label, match)
			}
		}
	}
}

func TestCanonicalDeveloperContractIsDiscoverable(t *testing.T) {
	for _, path := range []string{"README.md", "example/main.go"} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(content)
		for _, required := range []string{
			"OILPRICEAPI_KEY",
			"BRENT_CRUDE_USD",
			"https://www.oilpriceapi.com/pricing",
		} {
			if !strings.Contains(text, required) {
				t.Errorf("%s does not expose canonical developer fact %q", path, required)
			}
		}
	}

	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"https://api.oilpriceapi.com",
		"/v1/prices/latest?by_code=BRENT_CRUDE_USD",
		"https://api.oilpriceapi.com/product-facts.json",
	} {
		if !strings.Contains(string(readme), required) {
			t.Errorf("README.md does not expose canonical developer fact %q", required)
		}
	}
}
