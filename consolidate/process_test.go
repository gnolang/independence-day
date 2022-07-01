package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var test2_mnemonic = "hair stove window more scrap patient endorse left early pear lawn school loud divide vibrant family still bulk lyrics firm plate media critic dove"
var test2_address_gno = "g1fupfatmln5844rjafzp6d2vc825vav2x2kzaac"
var test2_address_cosmos = "cosmos1fupfatmln5844rjafzp6d2vc825vav2xe277uu"

var ledger_mnemonic = "month left venture toilet hub man hover topple rocket thunder school firm mesh equip uncover hospital penalty erosion tone make dawn excite silk aim"
var ledger_address_cosmos = "cosmos1fz9nhh7upfn9sv02f3ck4zsu8uqaesmupv6pv2"
var ledger_address_gno = "g1fz9nhh7upfn9sv02f3ck4zsu8uqaesmujsxzdw"

func TestConvertAddress(t *testing.T) {

	test2, err := convertAddress(test2_address_cosmos)
	assert.Equal(t, test2_address_gno, test2)

	ledger, err := convertAddress(ledger_address_cosmos)
	assert.Equal(t, ledger_address_gno, ledger)

	ledger, err = convertAddress(ledger_address_gno)
	assert.Error(t, err)

	//TODO: test multisig convertion
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

var a4 = Account{
	Address: "cosmos1zzzyklkaqafpe8200y7y6y3u9a3cehkr223223",
	Coins: []Coin{
		{Amount: "1", Denom: "uatom"},
	},
	Vote: "",
}

func TestQualify(t *testing.T) {
	// one
	var accounts = []Account{a1}

	dist, totalWeight := qualify(accounts)

	assert.Equal(t, 455794000000, dist[0].Weight)
	assert.Equal(t, 455794000000, totalWeight)

	//two
	accounts = append(accounts, a2)

	dist, totalWeight = qualify(accounts)

	assert.Equal(t, 455794000000, dist[0].Weight)
	assert.Equal(t, 8081636500000, dist[1].Weight)
	assert.Equal(t, 8537430500000, totalWeight)

}

func TestDistribute(t *testing.T) {

	var accounts = []Account{a1}

	dist, totalWeight := qualify(accounts)
	dist = distribute(dist, totalWeight)
	// get entire distribution
	assert.Equal(t, 455794000000, dist[0].Weight)
	assert.Equal(t, int64(TOTAL_AIRDROP), dist[0].Gnot.RoundInt64())

	//  a portion

	accounts = append(accounts, a2)
	dist, totalWeight = qualify(accounts)
	dist = distribute(dist, totalWeight)

	assert.Equal(t, 455794000000, dist[0].Weight)
	assert.Equal(t, 8081636500000, dist[1].Weight)
	assert.Equal(t, 8537430500000, totalWeight)

	assert.Equal(t, int64(45379567), dist[0].Gnot.RoundInt64())
	assert.Equal(t, int64(804620433), dist[1].Gnot.RoundInt64())

	// tiny portion
	accounts = append(accounts, a3)
	dist, totalWeight = qualify(accounts)
	dist = distribute(dist, totalWeight)

	assert.Equal(t, 455794000000, dist[0].Weight)
	assert.Equal(t, 8081636500000, dist[1].Weight)
	assert.Equal(t, 1, dist[2].Weight)
	assert.Equal(t, 8537430500001, totalWeight)

	assert.Equal(t, "45379567.072312286150000000", dist[0].Gnot.String())
	assert.Equal(t, "804620432.927588151650000000", dist[1].Gnot.String())
	assert.Equal(t, "0.000099561350000000", dist[2].Gnot.String())

}
