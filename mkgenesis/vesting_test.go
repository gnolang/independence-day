package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewVestingOffByDefault(t *testing.T) {
	t.Parallel()

	v, err := newVesting(0, 0, 4, "")
	if err != nil {
		t.Fatalf("newVesting: %v", err)
	}
	if v != nil {
		t.Fatalf("got %+v, want nil (vesting off)", v)
	}

	// A nil schedule must leave every line untouched.
	r := row{amount: 100, addr: "g1aaa", line: "g1aaa=100ugnot"}
	if got := v.apply(r); got != r.line {
		t.Fatalf("apply with vesting off = %q, want %q", got, r.line)
	}
}

func TestNewVestingRejectsBadInput(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ start, end, pct int64 }{
		"end without start": {0, 100, 4},
		"start without end": {100, 0, 4},
		"start equals end":  {100, 100, 4},
		"start after end":   {200, 100, 4},
		"negative pct":      {100, 200, -1},
		"pct above 100":     {100, 200, 101},
	} {
		if _, err := newVesting(tc.start, tc.end, tc.pct, ""); err == nil {
			t.Errorf("%s: got nil error, want failure", name)
		}
	}
}

// TestVestingApply pins the §132 schedule: 4% unlocked at start, so 96% vests.
func TestVestingApply(t *testing.T) {
	t.Parallel()

	v, err := newVesting(1780000000, 1843072000, 4, "g1exempt")
	if err != nil {
		t.Fatalf("newVesting: %v", err)
	}

	got := v.apply(row{amount: 632000000000000, addr: "g1big", line: "g1big=632000000000000ugnot"})
	want := "g1big=632000000000000ugnot;vesting=606720000000000ugnot,1780000000,1843072000"
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	// Exempt addresses keep a bare row (§136-138).
	bare := row{amount: 100, addr: "g1exempt", line: "g1exempt=100ugnot"}
	if got := v.apply(bare); got != bare.line {
		t.Errorf("exempt: got %q, want %q", got, bare.line)
	}

	// 4% of 1 ugnot truncates to 0 unlocked, so the whole 1 vests.
	dust := row{amount: 1, addr: "g1dust", line: "g1dust=1ugnot"}
	if got, want := v.apply(dust), "g1dust=1ugnot;vesting=1ugnot,1780000000,1843072000"; got != want {
		t.Errorf("dust: got %q, want %q", got, want)
	}

	// At 100% unlocked nothing vests, so no suffix is written at all.
	all, err := newVesting(1, 2, 100, "")
	if err != nil {
		t.Fatalf("newVesting: %v", err)
	}
	plain := row{amount: 500, addr: "g1all", line: "g1all=500ugnot"}
	if got := all.apply(plain); got != plain.line {
		t.Errorf("fully unlocked: got %q, want %q", got, plain.line)
	}
}

// TestParseRowIgnoresVestingSuffix makes sure a vested sheet still totals to
// the balances, not the balances plus their schedules.
func TestParseRowIgnoresVestingSuffix(t *testing.T) {
	t.Parallel()

	addr, amount, err := parseRow("g1big=632000000000000ugnot;vesting=606720000000000ugnot,1780000000,1843072000")
	if err != nil {
		t.Fatalf("parseRow: %v", err)
	}
	if addr != "g1big" || amount != 632000000000000 {
		t.Fatalf("got %q/%d, want g1big/632000000000000", addr, amount)
	}
}

// TestBuildWithVestingIsOptIn is the property the rollout depends on: passing
// no vesting flags produces exactly the file a build without the feature does.
func TestBuildWithVestingIsOptIn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	premine := filepath.Join(dir, "non-airdrop.txt")
	if err := os.WriteFile(premine, []byte("g1aaa=100ugnot\ng1bbb=50ugnot\n"), 0o644); err != nil {
		t.Fatalf("writing premine: %v", err)
	}
	genbalance := writeGz(t, filepath.Join(dir, "genbalance.txt.gz"), "src:g1ccc=25ugnot\n")

	build := func(name string, extra ...string) string {
		out := filepath.Join(dir, name)
		args := append([]string{"-genbalance", genbalance, "-premine", premine, "-out", out}, extra...)
		if err := runBuild(args); err != nil {
			t.Fatalf("build %s: %v", name, err)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		return string(b)
	}

	off := build("off.txt")
	if off != "g1aaa=100ugnot\ng1bbb=50ugnot\ng1ccc=25ugnot\n" {
		t.Fatalf("vesting off produced %q", off)
	}

	on := build("on.txt", "-vesting-start=1780000000", "-vesting-end=1843072000")
	for _, want := range []string{
		"g1aaa=100ugnot;vesting=96ugnot,1780000000,1843072000\n",
		"g1bbb=50ugnot;vesting=48ugnot,1780000000,1843072000\n",
		"g1ccc=25ugnot;vesting=24ugnot,1780000000,1843072000\n",
	} {
		if !strings.Contains(on, want) {
			t.Errorf("vesting on missing %q\ngot:\n%s", want, on)
		}
	}
}
