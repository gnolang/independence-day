package main

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseRow(t *testing.T) {
	t.Parallel()

	addr, amount, err := parseRow("g1abc=1234ugnot")
	if err != nil {
		t.Fatalf("parseRow: %v", err)
	}
	if addr != "g1abc" || amount != 1234 {
		t.Fatalf("got %q/%d, want g1abc/1234", addr, amount)
	}

	for _, bad := range []string{
		"g1abc1234ugnot", // no '='
		"g1abc=1234",     // no denom
		"=1234ugnot",     // no address
		"g1abc=-5ugnot",  // negative
		"g1abc=xugnot",   // not a number
		"g1abc=1234gnot", // wrong denom
	} {
		if _, _, err := parseRow(bad); err == nil {
			t.Errorf("parseRow(%q) = nil error, want failure", bad)
		}
	}
}

func TestSecondColonField(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"atone1src:g1dst=39ugnot": "g1dst=39ugnot",
		"g1dst=39ugnot":           "g1dst=39ugnot", // cut prints the whole line when there is no delimiter
		"a:b:c":                   "b",
	} {
		if got := secondColonField(in); got != want {
			t.Errorf("secondColonField(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReadPremineStripsComments(t *testing.T) {
	t.Parallel()

	path := writeTemp(t, "non-airdrop.txt", `# a leading comment

g1aaa=100ugnot

g1bbb=200ugnot # an inline comment
# g1ccc=300ugnot
`)

	totals := map[string]int64{}
	if err := readPremine(path, totals); err != nil {
		t.Fatalf("readPremine: %v", err)
	}

	want := map[string]int64{"g1aaa": 100, "g1bbb": 200}
	if !reflect.DeepEqual(totals, want) {
		t.Fatalf("got %v, want %v", totals, want)
	}
}

func TestAccumulateSumsDuplicateAddresses(t *testing.T) {
	t.Parallel()

	premine := writeTemp(t, "non-airdrop.txt", "g1shared=100ugnot\ng1only=1ugnot\n")

	totals := map[string]int64{}
	if err := readPremine(premine, totals); err != nil {
		t.Fatalf("readPremine: %v", err)
	}
	// The same address arriving from the airdrop must be added, not replaced —
	// unlike LeftMerge on the consuming side, which is last-write-wins.
	if err := accumulate(strings.NewReader("src:g1shared=25ugnot\n"), "genbalance", totals, secondColonField); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	if totals["g1shared"] != 125 {
		t.Fatalf("g1shared = %d, want 125 (100 premine + 25 airdrop)", totals["g1shared"])
	}
	if totals["g1only"] != 1 {
		t.Fatalf("g1only = %d, want 1", totals["g1only"])
	}
}

// TestSortRowsTieBreak pins the ordering `sort -t = -k 2 -n -r` produced:
// amount descending, and equal amounts falling back to a whole-line comparison
// that -r reverses too.
func TestSortRowsTieBreak(t *testing.T) {
	t.Parallel()

	rows := sortRows(map[string]int64{
		"g1jquc9": 669761901607,
		"g1n742q": 669761901607,
		"g1ps4ee": 669761901607,
		"g1qqqx3": 669761901607,
		"g1big":   632000000000000,
		"g1dust":  1,
	})

	var got []string
	for _, r := range rows {
		got = append(got, r.line)
	}

	want := []string{
		"g1big=632000000000000ugnot",
		"g1qqqx3=669761901607ugnot",
		"g1ps4ee=669761901607ugnot",
		"g1n742q=669761901607ugnot",
		"g1jquc9=669761901607ugnot",
		"g1dust=1ugnot",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestRenderReportLayout(t *testing.T) {
	t.Parallel()

	r := &report{
		rows:  2,
		total: 1332999998378908,
		top:   []string{"g1aaa=1332999998378907ugnot", "g1bbb=1ugnot"},
	}

	want := "# Genesis\n\n" +
		"## lines\n```\n2 balances.txt\n```\n\n" +
		"## sum\n```\n1332999998378908ugnot\n1332999998gnot\n```\n\n" +
		"## duplicate accounts\n```\n```\n\n" +
		"## top 100 accounts\n```\ng1aaa=1332999998378907ugnot\ng1bbb=1ugnot\n```\n\n"

	if got := r.render("balances.txt"); got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestGnot(t *testing.T) {
	t.Parallel()

	for in, want := range map[int64]string{
		1332999998378908: "1332999998.378908",
		1000000:          "1.000000",
		1:                "0.000001",
		0:                "0.000000",
	} {
		if got := gnot(in); got != want {
			t.Errorf("gnot(%d) = %q, want %q", in, got, want)
		}
	}
}

func writeGz(t *testing.T, path, content string) string {
	t.Helper()

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer f.Close()

	zw := gzip.NewWriter(f)
	if _, err := zw.Write([]byte(content)); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing %s: %v", path, err)
	}
	return path
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}
