package main

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
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

	expected := types.MustNewDecFromStr("1000007000000000.000000000000000000")
	delta := expected.Mul(types.NewDecWithPrec(1, 4)) // 0.01%
	diff := sum.Sub(expected).Abs()

	if diff.GT(delta) {
		t.Errorf("sum %s is not within 0.01%% of expected %s", sum.String(), expected.String())
	}
}
