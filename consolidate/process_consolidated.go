package main

import (
	"encoding/json"
	"fmt"
	"github.com/gnolang/gno/pkgs/bech32"
	"github.com/gnolang/gno/pkgs/crypto"
	"io/ioutil"
	"strconv"
	"strings"
)

func main() {
	bz, err := ioutil.ReadFile("snapshot_consolidated_10562840.json")
	if err != nil {
		panic(err)
	}
	var doc []interface{}
	err = json.Unmarshal(bz, &doc)
	if err != nil {
		panic(err)
	}

	for i := range doc {
		duatoms := 0
		item := doc[i].(map[string]interface{})
		coins := item["coins"].([]interface{})
		uatoms := 0
		for j := range coins {
			coin := coins[j].(map[string]interface{})
			denom := coin["denom"]
			amount := whole(coin["amount"].(string))
			switch denom {
			case "uatom":
				amount_i, err := strconv.Atoi(amount)
				if err != nil {
					panic(err)
				}
				uatoms += amount_i
			case "duatom":
				amount_i, err := strconv.Atoi(amount)

				if err != nil {
					panic(err)
				}
				duatoms = amount_i
				uatoms += amount_i
			default:
				// ignore ibc denoms.
			}

		}
		gnoAddress, err := convertAddress(item["address"].(string))

		if err != nil {

			panic(err)
		}

		vote := item["vote"].(string)
		gnot := distribute(vote, duatoms, uatoms)

		if gnot > 0 {

			fmt.Printf("%s:%d\n", gnoAddress, gnot)
		}
	}
}

// drops decimals
func whole(s string) string {
	idx := strings.Index(s, ".")
	if idx == -1 {
		return s
	} else {
		return s[:idx]
	}
}

func convertAddress(cosmosAddress string) (string, error) {

	bz, err := crypto.GetFromBech32(cosmosAddress, "cosmos")
	if err != nil {
		return "", err
	}

	gnoAddress, err2 := bech32.Encode("g", bz)

	if err2 != nil {

		return "", err2
	}

	return gnoAddress, nil

}
func distribute(vote string, duatoms int, uatoms int) (gnots int) {

	// rules for voting option
	// rules for delegation ammount
	if duatoms <= 0 {

		return 0

	}

	// rules for gnot calcuation : current rule 1:1,000,000

	// if division result < 0.5 round down
	// if division result > 0.5 roud up.
	gnots = (uatoms/100000 + 5) / 10

	return

}
