package oilpriceapi

// Storefront-claims guard.
//
// Every SDK (Node, Python, PHP, Go) rejects mutable plan and storefront claims
// from its published surfaces so a stale number cannot ship into a released
// module. Free tiers, quotas, request rates, uptime, prices, catalog totals and
// update cadence all change; the reviewed contract at productFactsContract is
// the only place those facts live. Public surfaces link to it instead.
//
// The rule set mirrors scripts/validate-storefront-claims.mjs in the Node SDK.
// Go surfaces scanned: README.md, CHANGELOG.md, everything under example/, and
// every non-test .go file at the module root (comments and string literals are
// what customers read on pkg.go.dev). scripts/ is tooling and is excluded, the
// same as the Node and PHP guards.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const productFactsContract = "https://api.oilpriceapi.com/product-facts.json"

// fixedDemoRatePattern is a superset of the Node "fixed demo rate" rule: it
// also catches rpm/rph/rpd shorthand and the hourly/daily adverb forms.
var fixedDemoRatePattern = regexp.MustCompile(`(?i)\b\d+\s+((requests?|reqs?\.?)\s*(((per|an?)\s+|/\s*)(minutes?|mins?|hours?|hrs?|days?)|(minutely|hourly|daily))|rp(m|h|d))\b`)

type storefrontRule struct {
	label   string
	pattern *regexp.Regexp
}

// storefrontBlockedClaims is an ordered list so failures print deterministically.
var storefrontBlockedClaims = []storefrontRule{
	{"real-time claim", regexp.MustCompile(`(?i)\breal[ -]?time\b`)},
	{"fixed catalog total", regexp.MustCompile(`(?i)\b\d+\+\s+(commodit|endpoint|tool|api)`)},
	{"fixed traffic total", regexp.MustCompile(`(?i)\b2m\+?\s+api requests`)},
	{"fixed update cadence", regexp.MustCompile(`(?i)\b(every|updated|refresh(ed)?)\s+(in\s+)?\d+\s+minutes\b|\b(updated|refresh(ed)?)\s+every\s+\d+`)},
	{"uptime or SLA", regexp.MustCompile(`(?i)\b\d+(\.\d+)?%\s+uptime\b|\bSLA\b`)},
	{"price comparison", regexp.MustCompile(`(?i)\bbloomberg\b|\b\d+(\.\d+)?%\s+less\s+cost\b`)},
	{"unreviewed plan name", regexp.MustCompile(`(?i)\bprofessional\+|\bprofessional\s+plan\b|\bstarter plan\b|\bscale tier\b`)},
	{"unreviewed plan price", regexp.MustCompile(`(?i)\$\d+(\.\d+)?\s*(/|per\s+)(mo(nth)?|year)\b`)},
	{"fixed allowance", regexp.MustCompile(`(?i)\b(1,000|100)\s+requests?(/month|\s+per month|\s+\(lifetime\))`)},
	{"quota promise", regexp.MustCompile(`(?i)\bdoes\s+not\s+consume.{0,40}\bquota\b|\bunlimited\s+(history|webhooks?|requests?|commodit)`)},
	{"fixed quota window", regexp.MustCompile(`(?i)\b(daily|weekly|monthly|yearly)\s+((api|request)\s+)?quota\b|\b((api|request)\s+)?quota\b.{0,40}\b(daily|weekly|monthly|yearly)\b`)},
	{"free-tier claim", regexp.MustCompile(`(?i)\bfree\s+tier\b|\bfree\s+api\s+key\b`)},
	{"free endpoint claim", regexp.MustCompile(`(?i)\b(endpoint|resource|api)\s+is\s+free\b|\bincluded\s+in\s+all\s+tiers\b`)},
	{"fixed query allowance", regexp.MustCompile(`(?i)\b\d[\d,]*\s+(station\s+)?quer(y|ies)\s*(/|per\s+)month\b`)},
	{"fixed demo rate", fixedDemoRatePattern},
	{"universal catalog", regexp.MustCompile(`(?i)\ball\s+(latest\s+)?prices\b|\ball\s+commodit`)},
}

// storefrontSurfaces lists every customer-readable file in the published module.
// Paths are returned relative to root and sorted.
func storefrontSurfaces(root string) ([]string, error) {
	var files []string
	for _, name := range []string{"README.md", "CHANGELOG.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			files = append(files, name)
		}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	// Root non-test Go files make up the published package. Generated and
	// unexported root files remain covered so a future public claim cannot
	// bypass this guard.
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}

	// Everything readable under example/ ships to customers verbatim.
	exampleRoot := filepath.Join(root, "example")
	if _, err := os.Stat(exampleRoot); err == nil {
		err := filepath.WalkDir(exampleRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			switch filepath.Ext(entry.Name()) {
			case ".go", ".md", ".txt":
				relative, relErr := filepath.Rel(root, path)
				if relErr != nil {
					return relErr
				}
				files = append(files, filepath.ToSlash(relative))
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Strings(files)
	return files, nil
}

// storefrontFailures returns one `file: label "match"` line per violation plus
// the README contract-link requirement, mirroring the Node guard's output.
func storefrontFailures(root string) ([]string, error) {
	files, err := storefrontSurfaces(root)
	if err != nil {
		return nil, err
	}

	var failures []string
	for _, file := range files {
		content, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			return nil, err
		}
		for _, rule := range storefrontBlockedClaims {
			if match := rule.pattern.Find(content); match != nil {
				failures = append(failures, fmt.Sprintf("%s: %s %q", file, rule.label, string(match)))
			}
		}
	}

	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		return nil, err
	}
	if !strings.Contains(string(readme), productFactsContract) {
		failures = append(failures, "README.md: reviewed product-facts contract is not linked")
	}
	return failures, nil
}

func TestStorefrontClaimsMatchReviewedContract(t *testing.T) {
	failures, err := storefrontFailures(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range failures {
		t.Errorf("%s; link to %s instead of restating the claim", failure, productFactsContract)
	}
}

func TestStorefrontSurfacesCoverPublishedFiles(t *testing.T) {
	surfaces, err := storefrontSurfaces(".")
	if err != nil {
		t.Fatal(err)
	}
	set := map[string]bool{}
	for _, surface := range surfaces {
		set[surface] = true
	}
	for _, want := range []string{"README.md", "CHANGELOG.md", "client.go", "types.go", "example/main.go"} {
		if !set[want] {
			t.Errorf("storefront surfaces do not include %s: %v", want, surfaces)
		}
	}
	for _, surface := range surfaces {
		if strings.HasSuffix(surface, "_test.go") || strings.HasPrefix(surface, "scripts/") {
			t.Errorf("storefront surfaces should not include tooling or tests, got %s", surface)
		}
	}
}

func TestStorefrontGuardDiscoversNewPublishedFiles(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"README.md":                  "Current limits are returned in response metadata.\n" + productFactsContract + "\n",
		"CHANGELOG.md":               "## [9.9.9]\n- Added the free tier quickstart.\n",
		"example/main.go":            "package main\n",
		"example/nested/README.md":   "Guaranteed 99.9% uptime.\n",
		"future_surface.go":          "package oilpriceapi\n// Limited to 20 requests/hour.\n",
		"future_surface_test.go":     "package oilpriceapi\n// Limited to 99 requests/hour.\n",
		"scripts/fixture/main.go":    "package main\n// Fixture allows 50 requests/day.\n",
		"example/nested/fixture.bin": "\x00\xff\xfe",
	}
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	failures, err := storefrontFailures(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`CHANGELOG.md: free-tier claim "free tier"`,
		`example/nested/README.md: uptime or SLA "99.9% uptime"`,
		`future_surface.go: fixed demo rate "20 requests/hour"`,
	}
	if strings.Join(failures, "\n") != strings.Join(want, "\n") {
		t.Fatalf("storefront failures mismatch\nwant:\n%s\ngot:\n%s", strings.Join(want, "\n"), strings.Join(failures, "\n"))
	}
}

func TestStorefrontGuardRequiresContractLink(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Limits come from response metadata.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	failures, err := storefrontFailures(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(failures) != 1 || failures[0] != "README.md: reviewed product-facts contract is not linked" {
		t.Fatalf("expected the missing contract link to fail, got %v", failures)
	}
}

// TestStorefrontBlockedClaimsFireOnEachRule proves every rule is live with one
// representative claim, and that reviewed wording passes.
func TestStorefrontBlockedClaimsFireOnEachRule(t *testing.T) {
	samples := map[string]string{
		"real-time claim":       "Real-time oil prices",
		"fixed catalog total":   "150+ commodities",
		"fixed traffic total":   "2M+ API requests served",
		"fixed update cadence":  "Prices updated every 5 minutes",
		"uptime or SLA":         "99.9% uptime guaranteed",
		"price comparison":      "80% less cost than Bloomberg",
		"unreviewed plan name":  "Available on the Starter plan",
		"unreviewed plan price": "From $29/month",
		"fixed allowance":       "1,000 requests/month included",
		"quota promise":         "Unlimited history access",
		"fixed quota window":    "Monthly quota reached.",
		"free-tier claim":       "Sign up for a free API key",
		"free endpoint claim":   "This endpoint is free and included in all tiers",
		"fixed query allowance": "500 station queries/month",
		"fixed demo rate":       "Limited to 20 requests per hour",
		"universal catalog":     "Returns all latest prices",
	}
	if len(samples) != len(storefrontBlockedClaims) {
		t.Fatalf("every rule needs a sample: %d rules, %d samples", len(storefrontBlockedClaims), len(samples))
	}
	for _, rule := range storefrontBlockedClaims {
		sample, ok := samples[rule.label]
		if !ok {
			t.Errorf("rule %q has no sample claim", rule.label)
			continue
		}
		if !rule.pattern.MatchString(sample) {
			t.Errorf("rule %q did not match its sample %q", rule.label, sample)
		}
	}

	reviewed := []string{
		"Current limits are returned in response metadata.",
		"Requests per hour are returned in response metadata.",
		"Refresh cadence varies by source, market hours, dataset, and plan.",
		"See /v1/forecasts/monthly for the monthly forecast endpoint.",
		"SDK version 1.5.2 requires Go 1.21.",
		"Source and ToolName are sent as the X-OPA-Source and X-OPA-Tool headers.",
		"The Source struct fields remain free string types.",
	}
	for _, text := range reviewed {
		for _, rule := range storefrontBlockedClaims {
			if match := rule.pattern.FindString(text); match != "" {
				t.Errorf("reviewed wording %q tripped %q on %q", text, rule.label, match)
			}
		}
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
		productFactsContract,
	} {
		if !strings.Contains(string(readme), required) {
			t.Errorf("README.md does not expose canonical developer fact %q", required)
		}
	}
}
