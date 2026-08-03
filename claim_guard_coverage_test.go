package oilpriceapi

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// discoverPublishedFiles independently walks the module and returns every file
// that is distributed to a reader outside this repository:
//
//   - Markdown, which is rendered on GitHub and on pkg.go.dev.
//   - Non-test Go sources, whose doc comments pkg.go.dev publishes verbatim.
//
// This deliberately does NOT reuse publicSurfaceFiles — it is the independent
// second opinion that the guard's own file set is complete.
func discoverPublishedFiles(t *testing.T) []string {
	t.Helper()

	skipDirs := map[string]bool{
		".git":       true,
		".github":    true,
		"scripts":    true,
		"testdata":   true,
		"vendor":     true,
		"node_modul": true,
	}

	var found []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != "." && skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		switch {
		case strings.HasSuffix(name, "_test.go"):
			return nil
		case strings.HasSuffix(name, ".go"), strings.HasSuffix(name, ".md"):
			found = append(found, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk module: %v", err)
	}
	sort.Strings(found)
	return found
}

// TestClaimGuardCoversEveryPublishedFile is the regression test for the
// "99.9% uptime" claim that shipped in v1.0.0 through v1.3.1.
//
// That claim was not a bad regex — the "uptime or SLA" pattern would have
// caught it. It shipped because the guard read a hand-maintained list of
// files, and a hand-maintained list silently stops covering the repository
// the moment someone adds a file to it. Enumerate instead of listing.
func TestClaimGuardCoversEveryPublishedFile(t *testing.T) {
	covered := map[string]bool{}
	for _, path := range publicSurfaceFiles(t) {
		covered[path] = true
	}

	var uncovered []string
	for _, path := range discoverPublishedFiles(t) {
		if !covered[path] {
			uncovered = append(uncovered, path)
		}
	}

	if len(uncovered) > 0 {
		t.Errorf("%d published file(s) are NOT scanned by the product-claim guard: %v\n"+
			"Every distributed file must be scanned — an unscanned file is how "+
			"the \"99.9%% uptime\" claim reached six tagged releases.",
			len(uncovered), uncovered)
	}
}

// TestClaimGuardDetectsPreviouslyShippedClaims pins the guard against the
// literal strings we are known to have distributed, so a future edit cannot
// loosen a pattern until it no longer matches the thing it exists to catch.
func TestClaimGuardDetectsPreviouslyShippedClaims(t *testing.T) {
	shipped := []struct {
		name string
		text string
	}{
		// README.md line 619, tags v1.0.0 - v1.3.1.
		{"go sdk readme v1.0.0-v1.3.1", "- **99.9% uptime** with enterprise-grade reliability"},
		{"uptime with sla suffix", "99.9% uptime SLA"},
		{"bare sla reference", "backed by our SLA"},
	}

	patterns := forbiddenClaimPatterns()
	for _, tc := range shipped {
		t.Run(tc.name, func(t *testing.T) {
			for _, pattern := range patterns {
				if pattern.MatchString(tc.text) {
					return
				}
			}
			t.Errorf("no forbidden-claim pattern matches previously shipped copy %q; "+
				"the guard has been weakened past the claim it exists to catch", tc.text)
		})
	}
}

// forbiddenClaimPatterns exposes the guard's patterns for pinning. It reads
// them from the same source TestPublicSurfaceClaimDrift uses.
func forbiddenClaimPatterns() []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, p := range forbiddenClaims {
		out = append(out, p)
	}
	return out
}
