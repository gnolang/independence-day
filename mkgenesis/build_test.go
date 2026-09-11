package main

import (
	"compress/gzip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Valid bech32 test addresses, shared across this package's tests.
//
// The hand-written sheet readers now decode every address, so the old
// placeholders ("g1aaa", "g1shared", …) no longer parse — which is precisely
// the point of the check. These are real 20-byte payloads (0x11…, 0x22…, 0x33…
// repeated) with correct checksums: they exercise the happy path without naming
// a real counterparty, and they stay recognisable in failure output.
const (
	testAddr1 = "g1zyg3zyg3zyg3zyg3zyg3zyg3zyg3zyg3jptpnt" // 0x11 × 20
	testAddr2 = "g1yg3zyg3zyg3zyg3zyg3zyg3zyg3zyg3zauw8mu" // 0x22 × 20
	testAddr3 = "g1xvenxvenxvenxvenxvenxvenxvenxven0ze6ww" // 0x33 × 20

	// testAddr1 with its first two payload characters transposed: same shape,
	// same length, broken checksum. This is the mistake the shape regex could
	// not see — and the realistic one, since counterparty addresses arrive by
	// email and get pasted in by hand.
	testAddrTransposed = "g1yzg3zyg3zyg3zyg3zyg3zyg3zyg3zyg3jptpnt"
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

	path := writeTemp(t, "non-airdrop.txt", "# a leading comment\n"+
		"\n"+
		testAddr1+"=100ugnot\n"+
		"\n"+
		testAddr2+"=200ugnot # an inline comment\n"+
		"# "+testAddr3+"=300ugnot\n")

	totals := map[string]entry{}
	if err := readPremine(path, totals); err != nil {
		t.Fatalf("readPremine: %v", err)
	}

	want := map[string]entry{testAddr1: {amount: 100}, testAddr2: {amount: 200}}
	if !reflect.DeepEqual(totals, want) {
		t.Fatalf("got %v, want %v", totals, want)
	}
}

func TestAccumulateSumsDuplicateAddresses(t *testing.T) {
	t.Parallel()

	premine := writeTemp(t, "non-airdrop.txt",
		testAddr1+"=100ugnot\n"+testAddr2+"=1ugnot\n")

	totals := map[string]entry{}
	if err := readPremine(premine, totals); err != nil {
		t.Fatalf("readPremine: %v", err)
	}
	// The same address arriving from the airdrop must be added, not replaced —
	// unlike LeftMerge on the consuming side, which is last-write-wins.
	if err := accumulate(strings.NewReader("src:"+testAddr1+"=25ugnot\n"),
		"genbalance", totals, secondColonField, false /*unlocked*/, false /*validateAddrs*/); err != nil {
		t.Fatalf("accumulate: %v", err)
	}

	if totals[testAddr1].amount != 125 {
		t.Fatalf("shared = %d, want 125 (100 premine + 25 airdrop)", totals[testAddr1].amount)
	}
	if totals[testAddr2].amount != 1 {
		t.Fatalf("only = %d, want 1", totals[testAddr2].amount)
	}
}

// TestHandWrittenSheetsRejectInvalidAddresses is the regression test for the
// gap this check closes: a transposed character in a counterparty address used
// to survive every check in this repository and land in the shipped genesis.
//
// It was caught eventually — gnoland's Balance.Parse decodes bech32 when the
// genesis is assembled — but "eventually" meant at the genesis ceremony rather
// than in CI, seconds after the paste.
func TestHandWrittenSheetsRejectInvalidAddresses(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		read func(string, map[string]entry) error
	}{
		{"non-airdrop.txt", readPremine},
		{"publicsale.txt", readPublicSale},
	} {
		tc := tc // go.mod says go 1.17: loop variables are shared across iterations
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := writeTemp(t, tc.name, testAddrTransposed+"=100ugnot\n")
			err := tc.read(path, map[string]entry{})
			if err == nil {
				t.Fatal("a checksum-invalid address was accepted")
			}
			if !strings.Contains(err.Error(), "not valid bech32") {
				t.Fatalf("error does not name the cause: %v", err)
			}
		})
	}
}

// TestHandWrittenSheetsRejectNonCanonicalAddresses covers the other half of
// checkAddr: an address that decodes to the right 20 bytes but is spelled in a
// foreign HRP. Consumers compare these as strings, so a `cosmos1…` spelling of
// the correct key is still the wrong row.
func TestHandWrittenSheetsRejectNonCanonicalAddresses(t *testing.T) {
	t.Parallel()

	// testAddr1's 20 bytes, encoded with the cosmos HRP.
	const cosmosForm = "cosmos1zyg3zyg3zyg3zyg3zyg3zyg3zyg3zyg3pahzj0"

	path := writeTemp(t, "non-airdrop.txt", cosmosForm+"=100ugnot\n")
	err := readPremine(path, map[string]entry{})
	if err == nil {
		t.Fatal("a non-canonical address was accepted")
	}
	if !strings.Contains(err.Error(), "canonical g1 form") {
		t.Fatalf("error does not name the cause: %v", err)
	}
}

// TestGenbalanceIsNotRevalidated documents the deliberate asymmetry in
// accumulate: genbalance's 3.26M rows are produced by allocate's addrKey and
// decoding them again costs ~3.4s on every build. The shipped artifact is still
// decoded in full, once, by allocate's TestGenesisFileTotal.
func TestGenbalanceIsNotRevalidated(t *testing.T) {
	t.Parallel()

	totals := map[string]entry{}
	if err := accumulate(strings.NewReader("src:"+testAddrTransposed+"=1ugnot\n"),
		"genbalance", totals, secondColonField, false /*unlocked*/, false /*validateAddrs*/); err != nil {
		t.Fatalf("genbalance rows must not be revalidated here: %v", err)
	}
	if totals[testAddrTransposed].amount != 1 {
		t.Fatal("row was dropped")
	}
}

// TestSortRowsTieBreak pins the ordering `sort -t = -k 2 -n -r` produced:
// amount descending, and equal amounts falling back to a whole-line comparison
// that -r reverses too.
func TestSortRowsTieBreak(t *testing.T) {
	t.Parallel()

	rows := sortRows(map[string]entry{
		"g1jquc9": {amount: 669761901607},
		"g1n742q": {amount: 669761901607},
		"g1ps4ee": {amount: 669761901607},
		"g1qqqx3": {amount: 669761901607},
		"g1big":   {amount: 632000000000000},
		"g1dust":  {amount: 1},
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
