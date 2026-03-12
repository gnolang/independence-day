package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/gnolang/gno/pkgs/bech32"
	"github.com/gnolang/gno/pkgs/crypto"
	osm "github.com/gnolang/gno/pkgs/os"
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

const (
	TOTAL_AIRDROP_ATOM            = 350000000
	TOTAL_AIRDROP_ATONE           = 231000000
	TOTAL_AIRDROP_CONTRIBS        = 119000000
	TOTAL_AIRDROP_NT              = 300000000
	TOTAL_AIRDROP_GOVDAO_FOUNDERS = 7000

	MULTISIG_NT1_ADDRESS    = "g1pxj9x5jkklzam9v76q7sn7grm0xnuj69qu7lmf" //nt1: nt llc + investors
	MULTISIG_NT2_ADDRESS    = "g1sp27hn785v3kud6cg9dnhrng7wzp9cnljffhcg" //nt2: special case handling for aib accounts
	MULTISIG_GOVDAO_ADDRESS = "g1rp7cmetn27eqlpjpc4vuusf8kaj746tysc0qgh" // govdao t1
)

var ibcEscrowAddress = map[string]bool{}
var excludedAddresses = map[string]bool{}

func init() {
	loadEscrowAddress()
	loadExcludedAddresses()
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
	bz, err = io.ReadAll(gzReader)
	if err != nil {
		panic(err)
	}

	accounts := []Account{}

	err = json.Unmarshal(bz, &accounts)
	if err != nil {
		panic(err)
	}

	atomDist, totalAtom := qualify(accounts)
	atomDistributed := distribute(atomDist, totalAtom, TOTAL_AIRDROP_ATOM)

	processNTMultisig(atomDistributed, "cosmos", aibCosmosAddrs)

	// Atone processing
	atoneDist, totalAtone := processAtone()
	atoneDistributed := distribute(atoneDist, totalAtone, TOTAL_AIRDROP_ATONE)

	processNTMultisig(atoneDistributed, "atone", aibAtoneAddrs)

	totalDist := mergeDistributions(atomDistributed, atoneDistributed)

	// Allocate contributions budget to GovDAO multisig
	totalDist[MULTISIG_GOVDAO_ADDRESS] = Distribution{
		Account: Account{
			Address: MULTISIG_GOVDAO_ADDRESS,
		},
		GnoAddress: MULTISIG_GOVDAO_ADDRESS,
		Ugnot:      types.NewDec(int64(TOTAL_AIRDROP_CONTRIBS) * 1000000),
	}

	// Allocate NT budget to NT main multisig
	totalDist[MULTISIG_NT1_ADDRESS] = Distribution{
		Account: Account{
			Address: MULTISIG_NT1_ADDRESS,
		},
		GnoAddress: MULTISIG_NT1_ADDRESS,
		Ugnot:      types.NewDec(int64(TOTAL_AIRDROP_NT) * 1000000),
	}

	// Allocate GovDAO founders budget (1000 GNOT each)
	for _, addr := range govdaoFounders {
		totalDist[addr] = Distribution{
			Account: Account{
				Address: addr,
			},
			GnoAddress: addr,
			Ugnot:      types.NewDec(int64(TOTAL_AIRDROP_GOVDAO_FOUNDERS/len(govdaoFounders)) * 1000000),
		}
	}

	// Create gzipped file
	outputFile, err := os.Create("genbalance.txt.gz")
	if err != nil {
		panic(err)
	}
	defer outputFile.Close()

	gw := gzip.NewWriter(outputFile)
	defer gw.Close()

	// Sort totalDist by Account.Address
	ordered := make([]Distribution, 0, len(totalDist))
	for _, d := range totalDist {
		ordered = append(ordered, d)
	}
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Account.Address < ordered[j].Account.Address
	})
	for _, d := range ordered {
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

var aibCosmosAddrs = []string{
	"cosmos15hmqrc245kryaehxlch7scl9d9znxa58qkpjet",
	"cosmos17g3gk5ymjt35wre4p57hfvmex36jcedtd3hfal",
	"cosmos17v7h4wdvjzkg09qmzyvf5w70tpnjgvekndfk4u",
	"cosmos1k8ca4pnvy8k5t22hmfzvyzl9v9d54vdvd9cryx",
	"cosmos12n3pqter204ks5mfzdtsz0hv2tr9cqmegnkc8r",
	"cosmos1pu9ssyptk3fym7hawerv5tnfqenr3c0d92hl7a",
	"cosmos1cxt79zavgr9qvqfx9hjsr9aqvpx7ftan8heqc6",
}

var govdaoFounders = []string{
	"g1us8428u2a5satrlxzagqqa5m6vmuze025anjlj", // Jae
	"g1manfred47kzduec920z88wfr64ylksmdcedlf5", // Manfred
	"g12vx7dn3dqq89mz550zwunvg4qw6epq73d9csay", // Dongowon
	"g1m0rgan0rla00ygmdmp55f5m0unvsvknluyg2a4", // Morgan
	"g127l4gkhk0emwsx5tmxe96sp86c05h8vg5tufzq", // Maxwell
	"g1e6gxg5tvc55mwsn7t7dymmlasratv7mkv0rap2", // Milos
	"g1mx4pum9976th863jgry4sdjzfwu03qan5w2v9j", // Ray
}

var aibAtoneAddrs = []string{
	"atone15hmqrc245kryaehxlch7scl9d9znxa58wka40n",
	"atone1k8ca4pnvy8k5t22hmfzvyzl9v9d54vdvr9yyj7",
	"atone12n3pqter204ks5mfzdtsz0hv2tr9cqmexn2l3m",

	"atone17g3gk5ymjt35wre4p57hfvmex36jcedtr3twt8", // derived from cosmos17g3gk5ymjt35wre4p57hfvmex36jcedtd3hfal
	"atone17v7h4wdvjzkg09qmzyvf5w70tpnjgvekad43ry", // derived from cosmos17v7h4wdvjzkg09qmzyvf5w70tpnjgvekndfk4u
	"atone1cxt79zavgr9qvqfx9hjsr9aqvpx7ftanfh98wz",
}

func processNTMultisig(dist map[string]Distribution, prefix string, addrs []string) {
	total := processAddrs(addrs, dist, prefix)
	dist[MULTISIG_NT2_ADDRESS] = Distribution{
		Account: Account{
			Address: MULTISIG_NT2_ADDRESS,
		},
		GnoAddress: MULTISIG_NT2_ADDRESS,
		Ugnot:      total,
	}

	fmt.Printf("total on multisig: %s\n", total.String())
}

func processAddrs(addrs []string, dist map[string]Distribution, prefix string) types.Dec {
	total := types.ZeroDec()
	for _, addr := range addrs {
		gaddr, err := convertAddress(addr, prefix)
		if err != nil {
			panic(err)
		}

		fmt.Printf("processing aib address %s with gno address %s\n", addr, gaddr)

		d, ok := dist[gaddr]
		if !ok {
			fmt.Printf("aib address %s not found in distribution\n", addr)
			continue
		}

		total = total.Add(d.Ugnot)
		delete(dist, gaddr)
	}

	return total
}

func mergeDistributions(dist1, dist2 map[string]Distribution) map[string]Distribution {
	merged := make(map[string]Distribution)
	for k, v1 := range dist1 {
		v2, ok := dist2[k]
		if ok {
			fmt.Printf("merging address %s from %s with weight %d and %s with weight %d \n",
				truncateMiddle(k, 15),
				truncateMiddle(v1.Account.Address, 15), v1.Weight,
				truncateMiddle(v2.Account.Address, 15), v2.Weight,
			)
			v1.Weight += v2.Weight
			v1.Ugnot = v1.Ugnot.Add(v2.Ugnot)
		}
		// note that we keep v1 Account only if they are the same gno address
		merged[k] = v1
	}

	// add remaining from dist2
	for k, v2 := range dist2 {
		if _, ok := dist1[k]; ok {
			continue
		}

		merged[k] = v2
	}

	return merged
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

func qualify(accounts []Account) (map[string]Distribution, int) {
	dist := make(map[string]Distribution)

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
		gnoAddress, err := convertAddress(a.Address, "cosmos")
		if err != nil {
			fmt.Printf("skipping address %s: %s\n", a.Address, err)
			continue
		}

		d := Distribution{
			Account:    a,
			GnoAddress: gnoAddress,
			Weight:     w,
			Ugnot:      types.ZeroDec(),
		}

		dist[gnoAddress] = d
		if w > 0 {
			total += w
		}

	}

	return dist, total
}

func distribute(dist map[string]Distribution, totalWeight int, totalTokens int64) map[string]Distribution {
	tWeight := types.NewDec(int64(totalWeight))
	tAirdrop := types.NewDec(totalTokens)

	for k, d := range dist {
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
		dist[k] = d
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

func convertAddress(cosmosAddress string, prefix string) (string, error) {
	bz, err := crypto.GetFromBech32(cosmosAddress, prefix)
	if err != nil {
		return "", err
	}

	if len(bz) != 20 {
		return "", fmt.Errorf("address %s has %d bytes, expected 20 bytes", cosmosAddress, len(bz))
	}

	gnoAddress, err2 := bech32.Encode("g", bz)
	if err2 != nil {
		return "", err2
	}

	return gnoAddress, nil
}

func skip(address string) bool {
	// skip excluded addresses
	if excludedAddresses[address] {
		return true
	}

	// skip ibc escrow address
	if ibcEscrowAddress[address] {
		// return true
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

func loadExcludedAddresses() {
	content := osm.MustReadFile("excluded.txt")
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		// Trim whitespace
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract address (before any comment or whitespace)
		// Format: cosmos1xxxxxx # comment
		parts := strings.Fields(line)
		if len(parts) > 0 {
			addr := parts[0]
			excludedAddresses[addr] = true
		}
	}
}

// truncateMiddle truncates a string to maxLen runes with "..." in the middle
func truncateMiddle(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	ellipsis := "..."
	ellipsisLen := len([]rune(ellipsis))

	if maxLen <= ellipsisLen {
		return ellipsis[:maxLen]
	}

	remaining := maxLen - ellipsisLen
	frontLen := (remaining + 1) / 2
	backLen := remaining - frontLen

	return string(runes[:frontLen]) + ellipsis + string(runes[len(runes)-backLen:])
}
