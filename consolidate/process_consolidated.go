package main

import (
	"encoding/json"
	"fmt"
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
				uatoms += amount_i
			default:
				// ignore ibc denoms.
			}

		}
		if uatoms > 0 {
			fmt.Printf("%s:%d\n", item["address"], uatoms)
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
