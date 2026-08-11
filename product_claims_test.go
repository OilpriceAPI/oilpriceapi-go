package oilpriceapi

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var fixedDemoRatePattern = regexp.MustCompile(`(?i)\b\d+\s+((requests?|reqs?\.?)\s*(((per|an?)\s+|/\s*)(minutes?|mins?|hours?|hrs?|days?)|(minutely|hourly|daily))|rp(m|h|d))\b`)

type claimViolation struct {
	path  string
	label string
	match string
}

func publicClaimFiles(root string) ([]string, error) {
	files := []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "example", "main.go"),
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	// Root non-test Go files make up the published package. Tooling under
	// scripts/ is intentionally excluded; generated and unexported root files
	// remain covered so a future public claim cannot bypass this guard.
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(root, name))
	}
	return files, nil
}

func forbiddenPublicClaims() map[string]*regexp.Regexp {
	return map[string]*regexp.Regexp{
		"fixed catalog total":   regexp.MustCompile(`(?i)\b\d+\+\s+(commodit|endpoint|api)`),
		"fixed update cadence":  regexp.MustCompile(`(?i)(updated|refresh(ed)?)\s+every\s+\d+|every\s+\d+\s+minutes`),
		"unreviewed plan name":  regexp.MustCompile(`(?i)professional\+|starter plan|scale tier`),
		"unreviewed plan price": regexp.MustCompile(`(?i)\$\d+(\.\d+)?\s*(/|per\s+)(mo(nth)?|year)`),
		"uptime or SLA":         regexp.MustCompile(`(?i)\b\d+(\.\d+)?%\s+uptime|\bSLA\b`),
		"price comparison":      regexp.MustCompile(`(?i)bloomberg|\d+(\.\d+)?%\s+less\s+cost`),
		"quota promise":         regexp.MustCompile(`(?i)does\s+not\s+consume.{0,40}quota|\bunlimited\b`),
		"fixed demo rate":       fixedDemoRatePattern,
		"universal catalog":     regexp.MustCompile(`(?i)\ball\s+(latest\s+)?prices\b|\ball\s+commodit`),
		"real-time claim":       regexp.MustCompile(`(?i)\breal[- ]time\b`),
		"free-tier claim":       regexp.MustCompile(`(?i)\bfree\s+tier\b`),
	}
}

func findClaimViolations(files []string, forbidden map[string]*regexp.Regexp) ([]claimViolation, error) {
	var violations []claimViolation
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for label, pattern := range forbidden {
			if match := pattern.Find(content); match != nil {
				violations = append(violations, claimViolation{path: path, label: label, match: string(match)})
			}
		}
	}
	return violations, nil
}

func TestPublicSurfaceClaimDrift(t *testing.T) {
	publicFiles, err := publicClaimFiles(".")
	if err != nil {
		t.Fatal(err)
	}
	violations, err := findClaimViolations(publicFiles, forbiddenPublicClaims())
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range violations {
		t.Errorf("%s contains %s %q; link to the reviewed product contract instead", violation.path, violation.label, violation.match)
	}
}

func TestPublicSurfaceClaimDiscoveryCoversNewPackageFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "example"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"README.md":              "Current limits are returned in response metadata.",
		"example/main.go":        "package main\n",
		"future_surface.go":      "package oilpriceapi\n// Limited to 20 requests/hour.\n",
		"future_surface_test.go": "package oilpriceapi\n// Limited to 99 requests/hour.\n",
	}
	for path, content := range files {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	publicFiles, err := publicClaimFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	violations, err := findClaimViolations(publicFiles, forbiddenPublicClaims())
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 1 || filepath.Base(violations[0].path) != "future_surface.go" || violations[0].label != "fixed demo rate" {
		t.Fatalf("expected only the new public package file to fail the fixed-rate guard, got %+v", violations)
	}
}

func TestFixedDemoRatePattern(t *testing.T) {
	tests := []struct {
		text  string
		match bool
	}{
		{"20 requests per hour", true},
		{"20 requests/hour", true},
		{"20 req/hour", true},
		{"20 requests an hour", true},
		{"20 req/hr", true},
		{"20 requests/hr", true},
		{"20 req/min", true},
		{"20 requests/min", true},
		{"20 requests/day", true},
		{"20 rph", true},
		{"20 rpm", true},
		{"20 rpd", true},
		{"20 requests hourly", true},
		{"20 requests daily", true},
		{"Current limits are returned in response metadata.", false},
		{"Requests per hour are returned in response metadata.", false},
		{"20 requests total", false},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			if got := fixedDemoRatePattern.MatchString(test.text); got != test.match {
				t.Fatalf("match=%v, want %v", got, test.match)
			}
		})
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
