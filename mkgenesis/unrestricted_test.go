package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUnrestrictedNamedFundsMatchAllocate is the anti-drift check. The three
// §127 funds are declared in allocate/process_consolidated.go and copied here,
// because the two are separate main packages. A copy that nobody verifies is a
// copy that goes stale, so this parses the real constants out of the source
// rather than trusting the copy.
//
// If the treasury split ever renames or repoints one of these, this fails here
// rather than silently shipping an exemption list that whitelists an address the
// genesis no longer funds.
func TestUnrestrictedNamedFundsMatchAllocate(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("../allocate/process_consolidated.go")
	require.NoError(t, err)

	for _, tc := range []struct{ constName, want string }{
		{"TREASURY_ECOSYSTEM_ADDRESS", unrestrictedEcosystem},
		{"INVESTORS_UNLOCKED_ADDRESS", unrestrictedInvestorsUnlocked},
		{"INVESTORS_VESTING_ADDRESS", unrestrictedInvestorsVesting},
	} {
		re := regexp.MustCompile(tc.constName + `\s*=\s*"(g1[0-9a-z]{38})"`)
		m := re.FindSubmatch(src)
		require.NotNil(t, m, "%s not found in allocate/process_consolidated.go", tc.constName)
		assert.Equal(t, tc.want, string(m[1]),
			"%s moved in allocate/ but not in mkgenesis/unrestricted.go", tc.constName)
	}
}

// TestUnrestrictedCoversEveryPublicSaleRow is the property that matters
// operationally: a sale participant whose row is in the genesis but whose
// address is not exempt receives GNOT they cannot move, and the contractual
// position ("lockup-free") is not honoured. The list is generated FROM
// publicsale.txt precisely so the two cannot disagree; this asserts it.
func TestUnrestrictedCoversEveryPublicSaleRow(t *testing.T) {
	t.Parallel()

	sale, err := readAddresses(publicSaleFileLocal)
	require.NoError(t, err)
	require.NotEmpty(t, sale)

	listed, err := readAddresses(unrestrictedFileLocal)
	require.NoError(t, err)

	set := make(map[string]bool, len(listed))
	for _, a := range listed {
		set[a] = true
	}
	for _, a := range sale {
		assert.True(t, set[a], "public-sale address %s is not in unrestricted.txt", a)
	}

	// ... and the three named funds, which are the whole point of §127.
	for _, a := range []string{unrestrictedEcosystem, unrestrictedInvestorsUnlocked, unrestrictedInvestorsVesting} {
		assert.True(t, set[a], "named fund %s is not in unrestricted.txt", a)
	}

	assert.Len(t, listed, len(sale)+3,
		"unrestricted.txt should be exactly the sale rows plus the three named funds")
}

// TestUnrestrictedIsRegenerated fails when the committed file is stale, the same
// way golden_test.go does for the balance sheet. The file is a public contract:
// gnolang/gno fetches it by pinned URL and sha256, so a stale copy in this repo
// becomes a wrong exemption list on a chain that cannot be edited afterwards.
func TestUnrestrictedIsRegenerated(t *testing.T) {
	t.Parallel()

	committed, err := os.ReadFile(unrestrictedFileLocal)
	require.NoError(t, err)

	tmp := t.TempDir() + "/unrestricted.txt"
	require.NoError(t, runUnrestricted([]string{"-publicsale", publicSaleFileLocal, "-out", tmp}))

	got, err := os.ReadFile(tmp)
	require.NoError(t, err)

	if string(got) != string(committed) {
		t.Fatalf("unrestricted.txt is stale — run `make -C mkgenesis unrestricted.txt` and commit\n"+
			"committed %d bytes, regenerated %d bytes", len(committed), len(got))
	}
	assert.True(t, strings.HasPrefix(string(committed), "# Unrestricted addresses"))
}

const (
	publicSaleFileLocal   = "publicsale.txt"
	unrestrictedFileLocal = "unrestricted.txt"
)
