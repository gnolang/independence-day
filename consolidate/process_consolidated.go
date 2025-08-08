package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/gnolang/gno/pkgs/bech32"
	"github.com/gnolang/gno/pkgs/crypto"
	osm "github.com/gnolang/gno/pkgs/os"

	"github.com/cosmos/cosmos-sdk/types"
)

type Account struct {
	Address string `json:"address"`
	Coins   []Coin `json:"coins"`
	Vote    string `json:"vote"`
}

type Coin struct {
	Amount string `json:"amount"`
	Denom  string `json:"denom"`
}

type Distribution struct {
	Account    Account   `json:"account"`
	GnoAddress string    `json:"gno_address"`
	Weight     int       `json:"weight"`
	Ugnot      types.Dec `json:"ugnot"`
}

// total 1,000,000,000 gnot
// Air drop 70%

const TOTAL_AIRDROP = 700000000

var ibcEscrowAddress = map[string]bool{}

func init() {
	loadEscrowAddress()
}

func main() {
	var bz []byte
	var err error
	var file *os.File
	var gzReader *gzip.Reader

	// Read the compressed file
	file, err = os.Open("snapshot_consolidated_10562840.json.gz")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Create a gzip reader
	gzReader, err = gzip.NewReader(file)
	if err != nil {
		panic(err)
	}
	defer gzReader.Close()

	// Read the decompressed content
	bz, err = ioutil.ReadAll(gzReader)
	if err != nil {
		panic(err)
	}

	accounts := []Account{}

	err = json.Unmarshal(bz, &accounts)
	if err != nil {
		panic(err)
	}

	dist, totalWeight := qualify(accounts)
	dist = distribute(dist, totalWeight)

	// Create gzipped file
	outputFile, err := os.Create("genbalance.txt.gz")
	if err != nil {
		panic(err)
	}
	defer outputFile.Close()

	gw := gzip.NewWriter(outputFile)
	defer gw.Close()

	for _, d := range dist {
		ugnot := whole(d.Ugnot.String())

		if ugnot != "0" {
			line := fmt.Sprintf("%s:%s=%sugnot\n", d.Account.Address, d.GnoAddress, ugnot)
			_, err := gw.Write([]byte(line))
			if err != nil {
				panic(err)
			}
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

// assign weight as uatom to each account and return the total weight

func qualify(accounts []Account) ([]Distribution, int) {
	dist := []Distribution{}

	total := 0
	for _, a := range accounts {

		if skip(a.Address) {
			continue
		}
		duatoms := 0
		uatoms := 0
		for _, c := range a.Coins {
			denom := c.Denom
			amount := whole(c.Amount)
			switch denom {

			case "uatom":
				amount_i, err := strconv.Atoi(amount)
				if err != nil {
					panic(err)
				}
				uatoms = amount_i
			case "duatom":
				amount_i, err := strconv.Atoi(amount)
				if err != nil {
					panic(err)
				}
				duatoms = amount_i

			default:
				// ignore ibc denoms.
			}

		}

		w := weight(a.Vote, uatoms, duatoms)
		gnoAddress, err := convertAddress(a.Address)
		if err != nil {
			panic(err)
		}

		d := Distribution{
			Account:    a,
			GnoAddress: gnoAddress,
			Weight:     w,
			Ugnot:      types.ZeroDec(),
		}

		dist = append(dist, d)
		if w > 0 {
			total += w
		}

	}

	return dist, total
}

func distribute(dist []Distribution, totalWeight int) []Distribution {
	tWeight := types.NewDec(int64(totalWeight))
	tAirdrop := types.NewDec(int64(TOTAL_AIRDROP))

	for i, d := range dist {
		/*
			// 1:1 mapping between weight and Ugnot token. It is easy to verify by users.
			// they don't need know total and percentage to know their own numebr based on rules.

			ugnot := types.NewDec(int64(d.Weight))
			d.Ugnot = ugnot
			dist[i] = d

		*/

		// propostional
		w := types.NewDec(int64(d.Weight))
		gnot := w.Quo(tWeight).Mul(tAirdrop)
		ugnot := gnot.Mul(types.NewDec(int64(1000000)))
		d.Ugnot = ugnot
		dist[i] = d

	}

	return dist
}

//  VOTE_OPTION_UNSPECIFIED = 0;
//  VOTE_OPTION_YES = 1;
//  VOTE_OPTION_ABSTAIN = 2;
//  VOTE_OPTION_NO = 3;
//  VOTE_OPTION_NO_WITH_VETO = 4;

func weight(vote string, uatom int, duatom int) int {
	weight := 0
	// rules for voting option
	if strings.Contains(vote, "\"option\":1") { // YES on Pro69

		duatom = 0
	} else if strings.Contains(vote, "\"option\":4") { // NO_WITH_VETO  on Pro69

		duatom = duatom * 2
	} else if strings.Contains(vote, "\"option\":3") { // NO on Pro69

		duatom = duatom + duatom>>1 //  * 1.5
	} else { // ABSTAIN, UNSPECIFIED, No voting options.

		// do nothing, they have the same weight as the delegated uatom.
	}

	weight = uatom + duatom

	return weight
}

func convertAddress(cosmosAddress string) (string, error) {
	// To debug, we can comment out this section and just return cosmos address

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

func skip(address string) bool {
	// skip ibc escrow address
	if ibcEscrowAddress[address] {
		// return true
	}

	//identify  and skip module account
	/*
	   cosmos1fl48vsnmsdzcv85q5d2q4z5ajdha8yu34mf0eh: 184285143502836 uatom
	   cosmos1tygms3xhhs3yv487phx3dw4a95jn7t7lpm470r: 9579821953422 uatom
	   cosmos1jv65s3grqf6v6jl3dp4t6c9t9rk99cd88lyufl: 5273424739633 uatom
	   cosmos17xpfvakm2amg962yls6f84z3kell8c5lserqta: 7616728 uatom
	*/

	module := []string{
		"cosmos1fl48vsnmsdzcv85q5d2q4z5ajdha8yu34mf0eh",
		"cosmos1tygms3xhhs3yv487phx3dw4a95jn7t7lpm470r",
		"cosmos1jv65s3grqf6v6jl3dp4t6c9t9rk99cd88lyufl",
		"cosmos17xpfvakm2amg962yls6f84z3kell8c5lserqta",
	}

	for _, v := range module {
		if address == v {
			return true
		}
	}

	return false
}

func loadEscrowAddress() {
	content := osm.MustReadFile("ibc_escrow_address.txt")
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// format:
		// cosmos1xxxxxx:g1xxxxxxxxxxxxxxxx:channel-1
		addr := strings.Split(line, ":")[0]

		ibcEscrowAddress[addr] = true
	}
}
