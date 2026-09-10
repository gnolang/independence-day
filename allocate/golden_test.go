package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadmeMatchesCommitted regenerates README.md from the committed
// genbalance.txt.gz and requires it to match the committed README byte for byte.
//
// It exists because the hand-written README drifted without anything noticing:
// by the time it was caught it claimed 3,262,457 rows against an artifact
// holding 3,262,351, described "ten synthetic rows" when there were thirteen,
// and gave an example row paying an address that had been split away three
// PRs earlier. Every one of those figures is now measured rather than typed,
// and this test is what keeps them measured — regenerating allocate/ without
// regenerating its README is a red build, exactly as it already is for
// mkgenesis/.
func TestReadmeMatchesCommitted(t *testing.T) {
	got := filepath.Join(t.TempDir(), "README.md")

	if err := runReadme([]string{"-genbalance", outputFile, "-out", got}); err != nil {
		t.Fatalf("readme: %v", err)
	}

	generated, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("reading generated README: %v", err)
	}
	committed, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatalf("reading committed README: %v", err)
	}

	if !bytes.Equal(generated, committed) {
		line, got, want := firstDifference(generated, committed)
		t.Fatalf("committed README.md is stale — run `cd allocate && go run . readme`\n"+
			"first difference at line %d:\n  committed: %q\n  generated: %q",
			line, want, got)
	}
}

// firstDifference reports the 1-indexed line where two reports diverge. A byte
// count alone is useless here: the figures that go stale are row counts and
// addresses, which usually drift without changing the length of the file.
func firstDifference(got, want []byte) (line int, gotLine, wantLine string) {
	g := strings.Split(string(got), "\n")
	w := strings.Split(string(want), "\n")

	shorter := len(g)
	if len(w) < shorter {
		shorter = len(w)
	}

	for i := 0; i < shorter; i++ {
		if g[i] != w[i] {
			return i + 1, g[i], w[i]
		}
	}
	return shorter + 1, "", ""
}
