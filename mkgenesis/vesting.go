package main

import (
	"fmt"
	"strconv"
	"strings"
)

// Vesting (Constitution §132-138).
//
// OFF by default. With -vesting-start unset the output is byte-identical to a
// build from before the vesting pass existed.
//
//	§132  "All Genesis $GNOT allocations vest on a common schedule: 4% unlocked
//	       on the day $GNOT becomes transferrable ... and a 4% unlock every
//	       subsequent month for 24 months."
//
// A continuous linear schedule over 24 months expresses that exactly, because
// 4 + 96*m/24 = 4(m+1): at month m the holder has 4(m+1)% unlocked either way.
// So the vesting portion is 96% of the balance, start is the transferability
// date and end is start + 24 months.
//
// The output grammar is the one `gnogenesis balances add` parses
// (gno.land/pkg/gnoland/balance.go):
//
//	<addr>=<coins>;vesting=<coins>,<start>,<end>
//
// This runs after the merge, not in allocate/, because the schedule is a
// function of an address's FINAL balance — known only once the premine has been
// merged and duplicate addresses summed.
type vesting struct {
	start     int64 // unix seconds; the day GNOT becomes transferrable
	end       int64 // unix seconds; start + 24 months
	unlockPct int64 // percent unlocked at start
	exempt    map[string]bool
}

// newVesting validates the flags and returns nil when vesting is off.
//
// exempt exists for §136-138 — "150,000,000 $GNOT from the Investors allocation
// will be unlocked at the mainnet launch" — which is only expressible once that
// tranche has an address of its own. While Investors and NT,LLC share one
// address, one address would need two schedules and the exception cannot be
// written down at all.
func newVesting(start, end, unlockPct int64, exempt string) (*vesting, error) {
	if start == 0 && end == 0 {
		return nil, nil
	}
	if start == 0 {
		return nil, fmt.Errorf("-vesting-start is required when -vesting-end is set")
	}
	if end == 0 {
		return nil, fmt.Errorf("-vesting-end is required when -vesting-start is set")
	}
	if start >= end {
		return nil, fmt.Errorf("-vesting-start (%d) must be before -vesting-end (%d)", start, end)
	}
	if unlockPct < 0 || unlockPct > 100 {
		return nil, fmt.Errorf("-vesting-unlock-pct must be 0..100, got %d", unlockPct)
	}

	v := &vesting{start: start, end: end, unlockPct: unlockPct, exempt: map[string]bool{}}
	for _, addr := range strings.FieldsFunc(exempt, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t' || r == '\n'
	}) {
		v.exempt[addr] = true
	}

	return v, nil
}

// apply returns the row's line with a vesting schedule appended, or the line
// unchanged when the address is exempt or nothing would vest.
//
// The arithmetic is int64 throughout. The awk version this replaced accumulated
// in float64 and had to refuse to run once amount*pct reached 2^53; that guard
// is no longer needed. The largest conceivable balance is the 1.333e15 ugnot
// total supply, and even that times 100 is still two orders of magnitude below
// int64's range.
func (v *vesting) apply(r row) string {
	if v == nil || v.exempt[r.addr] {
		return r.line
	}

	vested := r.amount - r.amount*v.unlockPct/100
	if vested <= 0 {
		return r.line
	}

	return r.line + ";vesting=" + strconv.FormatInt(vested, 10) + denom +
		"," + strconv.FormatInt(v.start, 10) +
		"," + strconv.FormatInt(v.end, 10)
}

func (v *vesting) describe() string {
	if v == nil {
		return "vesting: off"
	}
	return fmt.Sprintf("vesting: %d%% unlocked at %d, linear to %d, %d exempt",
		v.unlockPct, v.start, v.end, len(v.exempt))
}
