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
}

func TestDistribute(t *testing.T) {

	vote := "{\"option\":1,\"weight\":\"1.000000000000000000\"}"
	uatoms := 5083895 + 455794
	duatoms := 5083895

	// test >0.5 round up
	assert.Equal(t, 6, distribute(vote, duatoms, uatoms))

	vote = "{\"option\":1,\"weight\":\"1.000000000000000000\"}"
	uatoms = 5083895
	duatoms = 0

	// test not delegation no distribution
	assert.Equal(t, 0, distribute(vote, duatoms, uatoms))

}
