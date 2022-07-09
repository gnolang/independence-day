package main

import (
	"fmt"
	"github.com/cosmos/cosmos-sdk/types"
	osm "github.com/gnolang/gno/pkgs/os"
	"github.com/stretchr/testify/assert"
	"strconv"
	"strings"
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
	assert.Equal(t, "455794000000", whole(dist[0].Ugnot.String()))

	//  a portion

	accounts = append(accounts, a2)
	dist, totalWeight = qualify(accounts)
	dist = distribute(dist, totalWeight)

	assert.Equal(t, 455794000000, dist[0].Weight)
	assert.Equal(t, 8081636500000, dist[1].Weight)
	assert.Equal(t, 8537430500000, totalWeight)
	assert.Equal(t, "455794000000", whole(dist[0].Ugnot.String()))
	assert.Equal(t, "8081636500000", whole(dist[1].Ugnot.String()))

	// tiny portion
	accounts = append(accounts, a3)
	dist, totalWeight = qualify(accounts)
	dist = distribute(dist, totalWeight)

	assert.Equal(t, 455794000000, dist[0].Weight)
	assert.Equal(t, 8081636500000, dist[1].Weight)
	assert.Equal(t, 1, dist[2].Weight)
	assert.Equal(t, 8537430500001, totalWeight)

	assert.Equal(t, "455794000000", whole(dist[0].Ugnot.String()))
	assert.Equal(t, "8081636500000", whole(dist[1].Ugnot.String()))
	assert.Equal(t, "1", whole(dist[2].Ugnot.String()))

}
func TestTotal(t *testing.T) {

	bz := osm.MustReadFile("genbalance.txt")

	line := strings.TrimSuffix(string(bz), "\n")

	balances := strings.Split(line, "\n")

	sum := types.ZeroDec()

	for _, v := range balances {
		//cosmos10008uvk6fj3ja05u092ya5sx6fn355wavael4j:g10008uvk6fj3ja05u092ya5sx6fn355walp9u5k=3204884ugnot
		//split and drop cosmos address
		a := strings.Split(v, ":")
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
	assert.Equal(t, "300145508239404.000000000000000000", sum.String())

}
