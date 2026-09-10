package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestVestingApplyPublicSale covers the two things the public sale added to
// apply(): a genesis-liquid portion that the common schedule must not cover,
// and a row that brings its own schedule.
func TestVestingApplyPublicSale(t *testing.T) {
	t.Parallel()

	v, err := newVesting(1780000000, 1843072000, 4, "")
	if err != nil {
		t.Fatalf("newVesting: %v", err)
	}

	// A pure sale row: everything is liquid, so nothing vests and no suffix is
	// written — the same outcome as an exemption, without needing one.
	pure := row{amount: 1000, unlocked: 1000, addr: "g1sale", line: "g1sale=1000ugnot"}
	if got := v.apply(pure); got != pure.line {
		t.Errorf("pure sale: got %q, want %q", got, pure.line)
	}

	// The case an exempt-list cannot express: a sale participant who ALSO holds
	// an airdrop. 1000 total, 400 of it bought in the sale, so the schedule is
	// computed over the 600 airdrop only -> 96% of 600 = 576.
	both := row{amount: 1000, unlocked: 400, addr: "g1both", line: "g1both=1000ugnot"}
	if got, want := v.apply(both), "g1both=1000ugnot;vesting=576ugnot,1780000000,1843072000"; got != want {
		t.Errorf("sale+airdrop: got %q, want %q", got, want)
	}

	// A declared schedule is emitted verbatim and suppresses the common one.
	declared := row{
		amount:   1841860465,
		unlocked: 1841860465,
		schedule: ";vesting=1841860465ugnot,0,1820534400;type=delayed",
		addr:     "g1us",
		line:     "g1us=1841860465ugnot",
	}
	want := "g1us=1841860465ugnot;vesting=1841860465ugnot,0,1820534400;type=delayed"
	if got := v.apply(declared); got != want {
		t.Errorf("declared: got %q, want %q", got, want)
	}

	// ... and it SURVIVES vesting being off. This is the point: the default
	// build is vesting-off, so dropping it here is what put a forced-lockup
	// allocation into the shipped sheet as fully liquid, with only a line in the
	// build log to say so. A declared schedule is not part of the opt-in §132
	// mechanism -- it arrived with the input data because §132 cannot express
	// it, and for this row it is a legal obligation.
	var off *vesting
	if got := off.apply(declared); got != want {
		t.Errorf("declared with vesting off: got %q, want %q", got, want)
	}

	// The opt-in property still holds for everything that did NOT declare one.
	plain := row{amount: 100, addr: "g1p", line: "g1p=100ugnot"}
	if got := off.apply(plain); got != plain.line {
		t.Errorf("undeclared row with vesting off must be untouched: got %q", got)
	}
}

// TestAccumulateRejectsTwoSchedules locks in that a second declared schedule for
// the same address is an error. An address carries at most one schedule, so
// merging two has no correct answer and silently keeping the first would be the
// worst of the three options.
func TestAccumulateRejectsTwoSchedules(t *testing.T) {
	t.Parallel()

	totals := map[string]entry{}
	err := accumulate(strings.NewReader(
		"g1x=10ugnot;vesting=10ugnot,0,1\ng1x=20ugnot;vesting=20ugnot,0,2\n"),
		"sheet", totals, stripComment, true)
	if err == nil {
		t.Fatal("got nil error, want a rejection of the second schedule")
	}
	if !strings.Contains(err.Error(), "at most one") {
		t.Fatalf("unexpected error: %v", err)
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
		args := append([]string{"-genbalance", genbalance, "-premine", premine, "-publicsale", "", "-out", out}, extra...)
		if err := runBuild(args); err != nil {
			t.Fatalf("build %s: %v", name, err)
		}
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		return string(b)
	}

	// -no-vesting has to be asked for by name now: §132 covers every allocation,
	// so a schedule-less sheet is a violation rather than a default.
	off := build("off.txt", "-no-vesting")
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

// TestVestingExemptMatchesAllocate keeps the §136 exemption in step with the
// address that actually receives the tranche. The constant is declared in
// allocate/process_consolidated.go and copied here because the two are separate
// main packages; a copy nobody verifies goes stale, so this parses the real one
// out of the source instead of trusting the copy.
//
// If it ever drifts, the exemption would be applied to an address that holds
// nothing and the real 150,000,000 §136 tranche would vest — locking 96% of the
// one allocation the Constitution says is unlocked at mainnet.
func TestVestingExemptMatchesAllocate(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("../allocate/process_consolidated.go")
	require.NoError(t, err)

	re := regexp.MustCompile(`INVESTORS_UNLOCKED_ADDRESS\s*=\s*"(g1[0-9a-z]{38})"`)
	m := re.FindSubmatch(src)
	require.NotNil(t, m, "INVESTORS_UNLOCKED_ADDRESS not found in allocate/")
	assert.Equal(t, string(m[1]), genesisVestingExempt,
		"the §136 exempt address moved in allocate/ but not in mkgenesis/vesting.go")
}

// TestGenesisVestingSpansTwentyFourMonths pins the shipped schedule to §132's
// shape: it must start at the genesis timestamp and complete 24 calendar months
// later. A fat-fingered digit here is not visible in any total — supply is
// unchanged either way — so nothing else would catch it.
func TestGenesisVestingSpansTwentyFourMonths(t *testing.T) {
	t.Parallel()

	start := time.Unix(genesisVestingStart, 0).UTC()
	end := time.Unix(genesisVestingEnd, 0).UTC()

	assert.Equal(t, start.AddDate(0, 24, 0), end, "§132: fully vested 24 months after the mainnet")
	assert.Equal(t, 0, start.Hour()+start.Minute()+start.Second(), "genesis should be on a UTC midnight")
	assert.True(t, end.After(start))
}
