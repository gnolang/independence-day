package main

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"strconv"

	"github.com/cosmos/cosmos-sdk/types"
)

const PHOTON_TO_ATONE_RATIO = 7 // 1 atone = 7 photons

func processAtone() (map[string]Distribution, int) {
	as := parseAtoneAccounts("snapshot_consolidated_atone_6439117.json.gz")

	return qualifyAtone(as)
}

func parseAtoneAccounts(filename string) []Account {
	file, err := os.Open(filename)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		panic(err)
	}
	defer gzReader.Close()

	bz, err := io.ReadAll(gzReader)
	if err != nil {
		panic(err)
	}

	var accounts []Account
	err = json.Unmarshal(bz, &accounts)
	if err != nil {
		panic(err)
	}

	return accounts
}

func qualifyAtone(accounts []Account) (dist map[string]Distribution, total int) {
	dist = make(map[string]Distribution)
	for _, a := range accounts {
		if skip(a.Address) {
			continue
		}

		var totalCoins int
		for _, c := range a.Coins {
			ai, err := strconv.Atoi(c.Amount)
			if err != nil {
				panic(err)
			}

			if c.Denom == "uatone" {
				totalCoins += ai
			}

			if c.Denom == "duatone" {
				totalCoins += ai
			}

			if c.Denom == "uphoton" {
				totalCoins += ai / PHOTON_TO_ATONE_RATIO
			}
		}

		gnoAddress, err := convertAddress(a.Address, "atone")
		if err != nil {
			panic(err)
		}

		d := Distribution{
			Account:    a,
			GnoAddress: gnoAddress,
			Weight:     totalCoins,
			Ugnot:      types.ZeroDec(),
		}

		dist[gnoAddress] = d
		total += totalCoins
	}

	return
}
