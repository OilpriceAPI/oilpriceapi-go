package oilpriceapi

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestPublicSurfaceClaimDrift(t *testing.T) {
	publicFiles := []string{
		"README.md",
		"analytics.go",
		"client.go",
		"ei.go",
		"errors.go",
		"stream.go",
		"subscriptions.go",
		"types.go",
		"example/main.go",
	}
	forbidden := map[string]*regexp.Regexp{
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
