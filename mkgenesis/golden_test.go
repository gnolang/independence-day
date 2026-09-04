package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// TestBuildReproducesCommittedArtifacts rebuilds balances.txt and README.md
// from the committed inputs and requires them to match the committed outputs.
//
// This is the check that made the Go rewrite safe to land — it proved the port
// changed no byte of any committed file. It keeps earning its place afterwards:
// it fails whenever allocate/ is changed and mkgenesis/ is not regenerated,
// which is exactly how main went internally inconsistent between #45 and #47.
func TestBuildReproducesCommittedArtifacts(t *testing.T) {
	dir := t.TempDir()
	balances := filepath.Join(dir, "balances.txt")
	readme := filepath.Join(dir, "README.md")

	if err := runBuild([]string{
		"-genbalance", "../allocate/genbalance.txt.gz",
		"-premine", "non-airdrop.txt",
		"-out", balances,
	}); err != nil {
		t.Fatalf("build: %v", err)
	}

	t.Run("balances.txt", func(t *testing.T) {
		want := gunzip(t, "balances.txt.gz")

		got, err := os.ReadFile(balances)
		if err != nil {
			t.Fatalf("reading generated balances: %v", err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("generated balances.txt does not match balances.txt.gz\n got sha256 %s (%d bytes)\nwant sha256 %s (%d bytes)",
				sha256Hex(got), len(got), sha256Hex(want), len(want))
		}
		t.Logf("%d bytes, sha256 %s", len(got), sha256Hex(got))
	})

	t.Run("README.md", func(t *testing.T) {
		if err := runReadme([]string{"-balances", balances, "-out", readme}); err != nil {
			t.Fatalf("readme: %v", err)
		}

		got, err := os.ReadFile(readme)
		if err != nil {
			t.Fatalf("reading generated README: %v", err)
		}
		want, err := os.ReadFile("README.md")
		if err != nil {
			t.Fatalf("reading committed README: %v", err)
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("generated README.md does not match the committed one\n got %d bytes\nwant %d bytes", len(got), len(want))
		}
	})
}

func gunzip(t *testing.T, path string) []byte {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer f.Close()

	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	defer zr.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, zr); err != nil {
		t.Fatalf("decompressing %s: %v", path, err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
