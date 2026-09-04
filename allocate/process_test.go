package main

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	test2_mnemonic       = "hair stove window more scrap patient endorse left early pear lawn school loud divide vibrant family still bulk lyrics firm plate media critic dove"
	test2_address_gno    = "g1fupfatmln5844rjafzp6d2vc825vav2x2kzaac"
	test2_address_cosmos = "cosmos1fupfatmln5844rjafzp6d2vc825vav2xe277uu"
)

var (
	ledger_mnemonic       = "month left venture toilet hub man hover topple rocket thunder school firm mesh equip uncover hospital penalty erosion tone make dawn excite silk aim"
	ledger_address_cosmos = "cosmos1fz9nhh7upfn9sv02f3ck4zsu8uqaesmupv6pv2"
	ledger_address_gno    = "g1fz9nhh7upfn9sv02f3ck4zsu8uqaesmujsxzdw"
)

var (
	cosmos_address_gno = "g1zzzyklkaqafpe8200y7y6y3u9a3cehkrekdft4"
)

const TOTAL_AIRDROP_TESTS = 700000000

func TestFinalizedSupplyConstants(t *testing.T) {
	assert.Equal(t, 632000000, TOTAL_AIRDROP_NT+TOTAL_AIRDROP_NT_LLC)

	// The cap covers everything that lands in mkgenesis/balances.txt.gz, which
	// is the buckets PLUS the non-airdrop premine. The previous version of this
	// test summed only the buckets, which is why a 2,455,000 GNOT overshoot in
	// the shipped file could pass CI.
	assert.Equal(t, TOTAL_SUPPLY,
		TOTAL_AIRDROP_ATOM+
			TOTAL_AIRDROP_ATONE+
			TOTAL_AIRDROP_NT+
			TOTAL_AIRDROP_NT_LLC+
			TOTAL_AIRDROP_CONTRIBS+
			TOTAL_AIRDROP_GOVDAO_FOUNDERS+
			TOTAL_PREMINE_NON_AIRDROP,
	)
}

// TestPremineMatchesFile keeps TOTAL_PREMINE_NON_AIRDROP honest against the
// actual contents of mkgenesis/non-airdrop.txt.
func TestPremineMatchesFile(t *testing.T) {
	f, err := os.Open(nonAirdropFile)
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })

	var sum int64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(strings.Split(sc.Text(), "#")[0])
		if line == "" {
			continue
		}
		_, amount, ok := strings.Cut(line, "=")
		require.True(t, ok, "malformed premine line: %q", line)
		v, err := strconv.ParseInt(strings.TrimSuffix(amount, "ugnot"), 10, 64)
		require.NoError(t, err)
		sum += v
	}
	require.NoError(t, sc.Err())

	assert.Equal(t, int64(TOTAL_PREMINE_NON_AIRDROP)*1000000, sum,
		"TOTAL_PREMINE_NON_AIRDROP is out of date with %s", nonAirdropFile)
}

// TestGenesisFileTotal asserts the EXACT ugnot total of the file that
// gnolang/gno actually downloads. Nothing tested this before: TestTotal covers
// genbalance.txt.gz only, so mkgenesis/balances.txt.gz could be — and was —
// left stale by a constant change while CI stayed green.
//
// The expected figure is the cap minus the truncation residual, which is a
// property of the data and cannot be derived from the constants alone.
func TestGenesisFileTotal(t *testing.T) {
	const expected = int64(1332999998328067) // 1,333,000,000 GNOT − 1.671933 truncation

	f, err := os.Open(balancesFile)
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })

	zr, err := gzip.NewReader(f)
	require.NoError(t, err)
	t.Cleanup(func() { zr.Close() })

	var (
		sum   int64
		rows  int
		seen  = make(map[string]struct{})
		valid = regexp.MustCompile(`^g1[0-9a-z]{38}=[0-9]+ugnot$`)
	)
	sc := bufio.NewScanner(zr)
	for sc.Scan() {
		line := sc.Text()
		rows++
		require.True(t, valid.MatchString(line), "malformed row %d: %q", rows, line)

		addr, amount, _ := strings.Cut(line, "=")
		if _, dup := seen[addr]; dup {
			t.Fatalf("duplicate address %s", addr)
		}
		seen[addr] = struct{}{}

		v, err := strconv.ParseInt(strings.TrimSuffix(amount, "ugnot"), 10, 64)
		require.NoError(t, err)
		require.NotZero(t, v, "zero-balance row %d: %q", rows, line)
		sum += v
	}
	require.NoError(t, sc.Err())

	assert.Equal(t, expected, sum,
		"%s total is wrong — did you change a constant without running `make` from the repo root?",
		balancesFile)
	t.Logf("%s: %d rows, %d ugnot (%.6f GNOT)", balancesFile, rows, sum, float64(sum)/1e6)
}

// TestAssignRefusesToOverwrite locks in the guard at the three fixed-allocation
// call sites. Before it existed these were plain map writes, so a treasury or
// founder address that also held ATOM/ATONE would have had its snapshot
// entitlement silently replaced.
func TestAssignRefusesToOverwrite(t *testing.T) {
	dist := map[string]Distribution{}

	// An address with no prior entitlement is fine.
	assert.NotPanics(t, func() { assign(dist, test2_address_gno, 1000) })
	assert.Equal(t, "1000000000", whole(dist[test2_address_gno].Ugnot.String()))

	// A zero-valued placeholder is also fine — qualify() inserts those for
	// every snapshot address, including ones that qualify for nothing.
	dist2 := map[string]Distribution{
		ledger_address_gno: {Ugnot: types.ZeroDec()},
	}
	assert.NotPanics(t, func() { assign(dist2, ledger_address_gno, 1000) })

	// A real entitlement must not be silently discarded.
	dist3 := map[string]Distribution{
		ledger_address_gno: {
			Account: Account{Address: ledger_address_cosmos},
			Ugnot:   types.NewDec(42),
		},
	}
	assert.Panics(t, func() { assign(dist3, ledger_address_gno, 1000) })
}

// TestHardcodedAddressesAreValid covers every address written into the sheet
// without being derived from a snapshot.
func TestHardcodedAddressesAreValid(t *testing.T) {
	assert.NotPanics(t, validateHardcodedAddresses)

	for _, addr := range append([]string{
		MULTISIG_GOVDAO_ADDRESS, MULTISIG_NT1_ADDRESS, MULTISIG_NT2_ADDRESS,
	}, govdaoFounders...) {
		key, err := addrKey(addr)
		require.NoError(t, err, "address %s", addr)
		assert.Equal(t, addr, key, "address %s is not in canonical g1 form", addr)
	}

	assert.Len(t, govdaoFounders, 7, "TOTAL_AIRDROP_GOVDAO_FOUNDERS is divided by len(govdaoFounders)")
	assert.Zero(t, TOTAL_AIRDROP_GOVDAO_FOUNDERS%len(govdaoFounders),
		"founders budget must divide evenly, otherwise the remainder is silently dropped")
}

// TestExcludedTypesDefaultsToNoOp is the property that makes this mechanism safe
// to merge ahead of the policy decision it exists for.
func TestExcludedTypesDefaultsToNoOp(t *testing.T) {
	assert.Empty(t, loadExcludedTypes(),
		"policy/excluded-types.txt must ship with every pattern commented out")
	assert.Empty(t, specialExcluded,
		"no address may be excluded by class until a pattern is uncommented")
}

func TestTypePatternMatching(t *testing.T) {
	exact := typePattern{raw: "CEX", prefix: "cex"}
	assert.True(t, exact.matches("CEX"))
	assert.True(t, exact.matches("  cex  "), "trimmed and case-insensitive")
	assert.False(t, exact.matches("CEX or DEX"), "exact must not match a longer type")

	glob := typePattern{raw: "Upbit*", prefix: "upbit", glob: true}
	assert.True(t, glob.matches("Upbit #01 (Deposit)"))
	assert.True(t, glob.matches("Upbit #20 (Staking)"))
	assert.False(t, glob.matches("Bithumb #04"))

	blank := typePattern{raw: "?", prefix: "?"}
	assert.True(t, blank.matches("?"))
	assert.False(t, blank.matches(""))
}

// TestSpecialAccountsCSVIsParseable guards the loader against the file it reads:
// special-accounts.csv is hand-maintained, has ragged rows and at least one bare
// double quote, and a parse failure here would be a panic at init.
func TestSpecialAccountsCSVIsParseable(t *testing.T) {
	assert.NotPanics(t, loadSpecialAccountExclusions)
}

// TestSkipIsHRPAgnostic is the regression test for the DokiaCapital leak: every
// excluded.txt entry is written in cosmos1… form, but qualifyAtone() feeds skip()
// the atone1… encoding of the same 20-byte key.
func TestSkipIsHRPAgnostic(t *testing.T) {
	const (
		dokiaCosmos = "cosmos14lultfckehtszvzw4ehu0apvsr77afvyhgqhwh"
		dokiaAtone  = "atone14lultfckehtszvzw4ehu0apvsr77afvyegusc0"
		dokiaGno    = "g14lultfckehtszvzw4ehu0apvsr77afvyy5u50n"
	)

	kc, err := addrKey(dokiaCosmos)
	require.NoError(t, err)
	ka, err := addrKey(dokiaAtone)
	require.NoError(t, err)
	assert.Equal(t, kc, ka, "the two encodings must collapse to one key")
	assert.Equal(t, dokiaGno, kc)

	assert.True(t, skip(dokiaCosmos), "cosmos1 form must be excluded")
	assert.True(t, skip(dokiaAtone), "atone1 form must be excluded — this is the bug")
	assert.True(t, skip(dokiaGno), "g1 form must be excluded")

	// A 32-byte ICA address must not blow up.
	assert.False(t, skip("atone109450hc972uvgsmfrra7wfz4a7yzrvv8e8vky6wkucyaggxhw6aq8sq5ry"))
}

// TestIBCEscrowIsSkipped locks in the enforcement. The list was loaded but the
// skip was commented out, so 106 keyless accounts were funded.
func TestIBCEscrowIsSkipped(t *testing.T) {
	// channel-2, the Osmosis transfer escrow — the largest of the funded ones.
	const (
		escrowCosmos = "cosmos12k2pyuylm9t7ugdvz67h9pg4gmmvhn5vlx9j35"
		escrowGno    = "g12k2pyuylm9t7ugdvz67h9pg4gmmvhn5vv6e3ss"
	)

	key, err := addrKey(escrowCosmos)
	require.NoError(t, err)
	assert.Equal(t, escrowGno, key)

	assert.True(t, skip(escrowCosmos), "cosmos1 form must be skipped")
	assert.True(t, skip(escrowGno), "g1 form must be skipped")

	assert.Len(t, ibcEscrowAddress, 426, "one escrow account per channel, 0..425")
}

func TestConvertAddress(t *testing.T) {
	test2, err := convertAddress(test2_address_cosmos, "cosmos")
	assert.NoError(t, err)
	assert.Equal(t, test2_address_gno, test2)

	ledger, err := convertAddress(ledger_address_cosmos, "cosmos")
	assert.NoError(t, err)
	assert.Equal(t, ledger_address_gno, ledger)

	_, err = convertAddress(ledger_address_gno, "cosmos")
	assert.Error(t, err)

	// 32-byte ICA (Interchain Account) address must be rejected
	_, err = convertAddress("cosmos1jmjhr8y7u89yad0yvxua3ssa2d84qv706rxdw8qysramenyek8ws7y2683", "cosmos")
	assert.Error(t, err, "32-byte ICA addresses must be rejected")

	// TODO: test multisig convertion
}

func TestConvertAddressRejectsNon20ByteAddresses(t *testing.T) {
	// Valid 20-byte address should succeed
	addr, err := convertAddress(test2_address_cosmos, "cosmos")
	require.NoError(t, err)
	assert.Equal(t, test2_address_gno, addr)

	// 32-byte ICA address must be rejected
	_, err = convertAddress("cosmos1jmjhr8y7u89yad0yvxua3ssa2d84qv706rxdw8qysramenyek8ws7y2683", "cosmos")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "20 bytes")
}

func TestQualifySkipsInvalidAddresses(t *testing.T) {
	// An account with a 32-byte ICA address should be silently skipped
	icaAccount := Account{
		Address: "cosmos1jmjhr8y7u89yad0yvxua3ssa2d84qv706rxdw8qysramenyek8ws7y2683",
		Coins: []Coin{
			{Amount: "1000000", Denom: "uatom"},
		},
		Vote: "",
	}

	accounts := []Account{a1, icaAccount}
	dist, totalWeight := qualify(accounts)

	// Only a1 should be in the distribution; the ICA account must be skipped
	assert.Len(t, dist, 1)
	assert.Equal(t, 455794000000, totalWeight)
	_, exists := dist[test2_address_gno]
	assert.True(t, exists)
}

var a1 = Account{
	Address: "cosmos1fupfatmln5844rjafzp6d2vc825vav2xe277uu",
	Coins: []Coin{
		{Amount: "455794000000", Denom: "uatom"},
		{Amount: "5083895000000", Denom: "duatom"},
	},
	Vote: "{\"option\":1,\"weight\":\"1.000000000000000000\"}",
}

var a2 = Account{
	Address: "cosmos1fz9nhh7upfn9sv02f3ck4zsu8uqaesmupv6pv2",
	Coins: []Coin{
		{Amount: "455794000000", Denom: "uatom"},
		{Amount: "5083895000000", Denom: "duatom"},
	},
	Vote: "{\"option\":3,\"weight\":\"1.000000000000000000\"}",
}

var a3 = Account{
	Address: "cosmos1zzzyklkaqafpe8200y7y6y3u9a3cehkr223223",
	Coins: []Coin{
		{Amount: "1", Denom: "uatom"},
	},
	Vote: "",
}

func TestQualify(t *testing.T) {
	// one
	accounts := []Account{a1}

	dist, totalWeight := qualify(accounts)
	assert.Equal(t, 455794000000, dist[test2_address_gno].Weight)
	assert.Equal(t, 455794000000, totalWeight)

	// two
	accounts = append(accounts, a2)

	dist, totalWeight = qualify(accounts)
	assert.Equal(t, 455794000000, dist[test2_address_gno].Weight)
	assert.Equal(t, 8081636500000, dist[ledger_address_gno].Weight)
	assert.Equal(t, 8537430500000, totalWeight)
}

func TestDistribute(t *testing.T) {
	accounts := []Account{a1}

	dist, totalWeight := qualify(accounts)

	dist = distribute(dist, totalWeight, TOTAL_AIRDROP_TESTS)
	// get entire distribution, TOTAL_AIRDROP = 750Mgnot
	assert.Equal(t, 455794000000, dist[test2_address_gno].Weight)
	assert.Equal(t, int64(TOTAL_AIRDROP_TESTS*1000000), dist[test2_address_gno].Ugnot.RoundInt64())

	//  a portion

	accounts = append(accounts, a2)
	dist, totalWeight = qualify(accounts)
	dist = distribute(dist, totalWeight, TOTAL_AIRDROP_TESTS)
	assert.Equal(t, 455794000000, dist[test2_address_gno].Weight)
	assert.Equal(t, 8081636500000, dist[ledger_address_gno].Weight)
	assert.Equal(t, 8537430500000, totalWeight)
	assert.Equal(t, "37371408177202", whole(dist[test2_address_gno].Ugnot.String()))
	assert.Equal(t, "662628591822797", whole(dist[ledger_address_gno].Ugnot.String()))
	// tiny portion
	accounts = append(accounts, a3)
	dist, totalWeight = qualify(accounts)
	dist = distribute(dist, totalWeight, TOTAL_AIRDROP_TESTS)
	assert.Equal(t, 455794000000, dist[test2_address_gno].Weight)
	assert.Equal(t, 8081636500000, dist[ledger_address_gno].Weight)
	assert.Equal(t, 1, dist[cosmos_address_gno].Weight)
	assert.Equal(t, 8537430500001, totalWeight)

	assert.Equal(t, "37371408177198", whole(dist[test2_address_gno].Ugnot.String()))
	assert.Equal(t, "662628591822719", whole(dist[ledger_address_gno].Ugnot.String()))
	assert.Equal(t, "81", whole(dist[cosmos_address_gno].Ugnot.String()))
}

func TestTotal(t *testing.T) {
	bz, err := os.Open("genbalance.txt.gz")
	require.NoError(t, err)

	zbz, err := gzip.NewReader(bz)
	require.NoError(t, err)

	t.Cleanup(func() {
		zbz.Close()
		bz.Close()
	})

	br := bufio.NewReader(zbz)

	sum := types.ZeroDec()
	for {
		line, err := br.ReadString('\n')
		if errors.Is(err, io.EOF) {
			break
		}

		require.NoError(t, err)
		line = strings.TrimSuffix(line, "\n")

		// cosmos10008uvk6fj3ja05u092ya5sx6fn355wavael4j:g10008uvk6fj3ja05u092ya5sx6fn355walp9u5k=3204884ugnot
		// split and drop cosmos address
		a := strings.Split(line, ":")
		parts := strings.Split(a[1], "=")
		if len(parts) != 2 {
			fmt.Printf("error in parsing: %v\n", parts)
		}

		amount := strings.TrimSuffix(parts[1], "ugnot")

		amount_i, err := strconv.Atoi(amount)
		if err != nil {
			panic(err)
		}

		amount_dec := types.NewDec(int64(amount_i))
		sum = sum.Add(amount_dec)
	}

	// genbalance.txt.gz carries the buckets only; the premine is added later by
	// mkgenesis. Derived from the constants so that flipping
	// PREMINE_ABSORBED_FROM_CONTRIBS does not silently break this test.
	// The exact total of the SHIPPED file is asserted by TestGenesisFileTotal.
	expected := types.NewDec(int64(TOTAL_SUPPLY-TOTAL_PREMINE_NON_AIRDROP) * 1000000)
	delta := expected.Mul(types.NewDecWithPrec(1, 4)) // 0.01%
	diff := sum.Sub(expected).Abs()

	if diff.GT(delta) {
		t.Errorf("sum %s is not within 0.01%% of expected %s", sum.String(), expected.String())
	}
}
