package main

import (
	"compress/gzip"
	"encoding/csv"
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

// Input files, relative to this directory. Snapshots and human-edited policy
// files live outside allocate/ so that "what we were given" and "what we
// decided" stay visibly separate from "how it is computed".
//
//	inputs/  — immutable chain snapshots, never edited after capture
//	policy/  — human-editable decisions (exclusions, annotations)
const (
	cosmosSnapshotFile  = "../inputs/cosmoshub-10562840.json.gz"
	atoneSnapshotFile   = "../inputs/atomone-6439117.json.gz"
	excludedFile        = "../policy/excluded.txt"
	excludedTypesFile   = "../policy/excluded-types.txt"
	specialAccountsFile = "../policy/special-accounts.csv"
	ibcEscrowFile       = "../policy/ibc-escrow-addresses.txt"

	// outputFile is consumed by mkgenesis/Makefile.
	outputFile = "genbalance.txt.gz"

	// Downstream artifacts, asserted by the tests in this package.
	nonAirdropFile = "../mkgenesis/non-airdrop.txt"
	balancesFile   = "../mkgenesis/balances.txt.gz"
)

// TOTAL_SUPPLY is the finalized cap. It covers everything that ends up in
// mkgenesis/balances.txt.gz — the buckets below AND the non-airdrop premine.
const TOTAL_SUPPLY = 1333000000

// TOTAL_PREMINE_NON_AIRDROP is the sum of mkgenesis/non-airdrop.txt: two
// faucets, three named contributors and 45 GitHub requesters. The finalized
// 7-bucket breakdown does not budget for it, so somebody has to pay for it.
//
// Was 2455000 until the test1 and test2 rows were removed on 2026-09-03 — both
// were funded from mnemonics published in gnolang/gno's test fixtures.
//
// TestPremineMatchesFile asserts this constant against the actual file.
const TOTAL_PREMINE_NON_AIRDROP = 2345000

// PREMINE_ABSORBED_FROM_CONTRIBS decides who pays for the premine above.
// This is the ONE LINE to flip; everything else follows.
//
//	Option A  = 0        AND empty mkgenesis/non-airdrop.txt
//	            -> drops all 48 remaining contributors' rows.
//	Option B  = TOTAL_PREMINE_NON_AIRDROP  (SELECTED) GovDAO Contributions absorbs it
//	            -> 1,332,999,998.378908 GNOT. Everyone keeps their row.
//	Option C  = 0        keep the file, accept the overshoot
//	            -> TOTAL_PREMINE_NON_AIRDROP GNOT over the cap.
//
// (The residual 1.621092 below the cap is per-account Dec->int truncation across
// 3.26M rows, in whole(). Deterministic and unavoidable without changing it.)
const PREMINE_ABSORBED_FROM_CONTRIBS = TOTAL_PREMINE_NON_AIRDROP // option B

const (
	TOTAL_AIRDROP_ATOM  = 350000000
	TOTAL_AIRDROP_ATONE = 231000000
	// §123-124 Investors 300M + NT,LLC 332M; §136-138 carves 150M of Investors
	// as unlocked at mainnet, which needs an address of its own because one
	// account carries one vesting schedule.
	TOTAL_INVESTORS_UNLOCKED = 150000000
	TOTAL_INVESTORS_VESTING  = 150000000
	TOTAL_AIRDROP_NT         = TOTAL_INVESTORS_UNLOCKED + TOTAL_INVESTORS_VESTING
	TOTAL_AIRDROP_NT_LLC     = 332000000

	// The three treasuries of Constitution §120-122, each with its own address.
	// They used to be one undifferentiated TOTAL_AIRDROP_CONTRIBS line.
	//
	//   §120  Core Treasury                 40,000,000
	//   §121  Ecosystem Treasury            60,000,000
	//   §122  Validator Services Treasury   20,000,000
	//                                      ------------
	//                                      120,000,000
	//
	// The premine and the founders allocation are paid OUT of these, so the
	// amounts actually written to the three addresses are net of them. Which
	// treasury absorbs which is a policy choice — see the two constants below.
	TOTAL_TREASURY_CORE      = 40000000
	TOTAL_TREASURY_ECOSYSTEM = 60000000
	TOTAL_TREASURY_VALIDATOR = 20000000

	TOTAL_AIRDROP_GOVDAO_FOUNDERS = 7000

	// §333: "Present GovDAO members are not eligible for any allocation from the
	// Ecosystem Treasury genesis allocation." All seven founders are GovDAO
	// founders, so the founders allocation is charged to CORE. While the three
	// treasuries shared one line this could not be shown either way; now it can.
	FOUNDERS_CHARGED_TO_CORE = TOTAL_AIRDROP_GOVDAO_FOUNDERS

	// §121 makes the Ecosystem Treasury "for prior and future Gno.land ecosystem
	// development" and §140 makes GovDAO responsible for distributing it "to
	// prior and future Gno.land ecosystem contributors" — so the 2022 contributor
	// premine is charged to ECOSYSTEM.
	//
	// NOTE this includes 2,000,000 GNOT of faucet funding, which is chain
	// operations rather than ecosystem development and arguably does not belong
	// in this treasury at all under §226. Left here because moving it needs a
	// decision about where the faucet IS funded from.
	PREMINE_CHARGED_TO_ECOSYSTEM = TOTAL_PREMINE_NON_AIRDROP

	// Net amounts written to the three treasury addresses.
	TOTAL_TREASURY_CORE_NET      = TOTAL_TREASURY_CORE - FOUNDERS_CHARGED_TO_CORE
	TOTAL_TREASURY_ECOSYSTEM_NET = TOTAL_TREASURY_ECOSYSTEM - PREMINE_CHARGED_TO_ECOSYSTEM
	TOTAL_TREASURY_VALIDATOR_NET = TOTAL_TREASURY_VALIDATOR

	MULTISIG_NT2_ADDRESS = "g1sp27hn785v3kud6cg9dnhrng7wzp9cnljffhcg" //nt2: special case handling for aib accounts

	INVESTORS_UNLOCKED_ADDRESS = "TODO_INVESTORS_UNLOCKED_ADDR" // no vesting schedule, §136
	INVESTORS_VESTING_ADDRESS  = "TODO_INVESTORS_VESTING_ADDR"
	NT_LLC_ADDRESS             = "TODO_NT_LLC_ADDR"

	// PLACEHOLDERS — these addresses do not exist yet. validateHardcodedAddresses()
	// refuses to run until they are replaced, so this cannot ship by accident.
	TREASURY_CORE_ADDRESS      = "TODO_CORE_TREASURY_ADDR"
	TREASURY_ECOSYSTEM_ADDRESS = "TODO_ECOSYSTEM_TREASURY_ADDR"
	TREASURY_VALIDATOR_ADDRESS = "TODO_VALIDATOR_TREASURY_ADDR"
)

var ibcEscrowAddress = map[string]bool{}
var excludedAddresses = map[string]bool{}

// specialExcluded holds addresses removed by CLASS via policy/excluded-types.txt,
// keyed by the canonical g1 form. Empty unless a pattern is uncommented there.
var specialExcluded = map[string]bool{}

func init() {
	validateHardcodedAddresses()
	loadEscrowAddress()
	loadExcludedAddresses()
	loadSpecialAccountExclusions()
}

func main() {
	var bz []byte
	var err error
	var file *os.File
	var gzReader *gzip.Reader

	// Read the compressed file
	file, err = os.Open(cosmosSnapshotFile)
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

	// Allocate each treasury to its own address (Constitution §120-122).
	assign(totalDist, TREASURY_CORE_ADDRESS, TOTAL_TREASURY_CORE_NET)
	assign(totalDist, TREASURY_ECOSYSTEM_ADDRESS, TOTAL_TREASURY_ECOSYSTEM_NET)
	assign(totalDist, TREASURY_VALIDATOR_ADDRESS, TOTAL_TREASURY_VALIDATOR_NET)

	// Allocate Investors and NT,LLC separately (§123-124), with the §136
	// mainnet-unlocked tranche split from the vesting remainder.
	assign(totalDist, INVESTORS_UNLOCKED_ADDRESS, TOTAL_INVESTORS_UNLOCKED)
	assign(totalDist, INVESTORS_VESTING_ADDRESS, TOTAL_INVESTORS_VESTING)
	assign(totalDist, NT_LLC_ADDRESS, TOTAL_AIRDROP_NT_LLC)

	// Allocate GovDAO founders budget (1000 GNOT each)
	for _, addr := range govdaoFounders {
		assign(totalDist, addr, TOTAL_AIRDROP_GOVDAO_FOUNDERS/len(govdaoFounders))
	}

	// Create gzipped file
	outputFile, err := os.Create(outputFile)
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

// assign writes a fixed allocation of `gnot` GNOT to `addr`.
//
// It PANICS if addr already carries an entitlement. The three call sites used to
// be plain map assignments, which silently discarded whatever was there. None of
// the ten hardcoded addresses currently appears in either snapshot, so nothing is
// being lost today — but that is a property of the input data, not of the code,
// and it would stop being true the moment a treasury or founder address turned
// out to hold ATOM or ATONE. A snapshot-derived entitlement would vanish with no
// diagnostic and no change in total supply, because the fixed amount replaces it.
//
// If this ever fires, the fix is a decision (does the address keep its airdrop on
// top of its allocation, or not?), not a code change — so it must not be silent.
func assign(dist map[string]Distribution, addr string, gnot int) {
	if existing, ok := dist[addr]; ok && !existing.Ugnot.IsZero() {
		panic(fmt.Errorf(
			"refusing to overwrite an existing entitlement: %s already holds %s ugnot "+
				"(from source address %s) and would be replaced by a fixed allocation of %d GNOT; "+
				"decide explicitly whether the two should be summed",
			addr, whole(existing.Ugnot.String()), existing.Account.Address, gnot))
	}

	dist[addr] = Distribution{
		Account:    Account{Address: addr},
		GnoAddress: addr,
		Ugnot:      types.NewDec(int64(gnot) * 1000000),
	}
}

// validateHardcodedAddresses checks every address this program writes into the
// balance sheet without deriving it from a snapshot. A malformed one would
// otherwise be discovered by whatever consumes the output — or, worse, not be
// discovered, since nothing downstream asserts the address format.
func validateHardcodedAddresses() {
	seen := make(map[string]string, len(govdaoFounders)+3)

	check := func(addr, role string) {
		if _, err := addrKey(addr); err != nil {
			panic(fmt.Errorf("%s: invalid address %q: %w", role, addr, err))
		}
		if prev, dup := seen[addr]; dup {
			panic(fmt.Errorf("address %s is used for both %s and %s", addr, prev, role))
		}
		seen[addr] = role
	}

	check(TREASURY_CORE_ADDRESS, "TREASURY_CORE_ADDRESS")
	check(TREASURY_ECOSYSTEM_ADDRESS, "TREASURY_ECOSYSTEM_ADDRESS")
	check(TREASURY_VALIDATOR_ADDRESS, "TREASURY_VALIDATOR_ADDRESS")
	check(INVESTORS_UNLOCKED_ADDRESS, "INVESTORS_UNLOCKED_ADDRESS")
	check(INVESTORS_VESTING_ADDRESS, "INVESTORS_VESTING_ADDRESS")
	check(NT_LLC_ADDRESS, "NT_LLC_ADDRESS")
	check(MULTISIG_NT2_ADDRESS, "MULTISIG_NT2_ADDRESS")
	for i, addr := range govdaoFounders {
		check(addr, fmt.Sprintf("govdaoFounders[%d]", i))
	}
}

// addrKey returns the canonical g1… form of a 20-byte bech32 address, whatever
// its human-readable part.
func addrKey(address string) (string, error) {
	_, bz, err := bech32.Decode(address)
	if err != nil {
		return "", err
	}
	if len(bz) != 20 {
		return "", fmt.Errorf("address %s has %d bytes, expected 20 bytes", address, len(bz))
	}
	return bech32.Encode("g", bz)
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
	// Skip excluded addresses. Matched on the 20-byte payload, not the bech32
	// string: excluded.txt is written in cosmos1… form and qualifyAtone() passes
	// the atone1… encoding of the same keys.
	if key, err := addrKey(address); err == nil && excludedAddresses[key] {
		return true
	}

	// skip addresses excluded by class via policy/excluded-types.txt
	if len(specialExcluded) > 0 {
		if key, err := addrKey(address); err == nil && specialExcluded[key] {
			return true
		}
	}

	// Skip IBC transfer escrow accounts. Derived as
	// sha256("ics20-1" || 0x00 || "transfer/channel-N")[:20] — no private key
	// exists for them on any chain, so crediting them is a burn.
	if key, err := addrKey(address); err == nil && ibcEscrowAddress[key] {
		return true
	}

	return false
}

func loadEscrowAddress() {
	content := osm.MustReadFile(ibcEscrowFile)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		// format:
		// cosmos1xxxxxx:g1xxxxxxxxxxxxxxxx:channel-1
		addr := strings.Split(line, ":")[0]

		key, err := addrKey(addr)
		if err != nil {
			panic(fmt.Errorf("%s: bad address %q: %w", ibcEscrowFile, addr, err))
		}
		ibcEscrowAddress[key] = true
	}
}

func loadExcludedAddresses() {
	content := osm.MustReadFile(excludedFile)
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
			key, err := addrKey(parts[0])
			if err != nil {
				panic(fmt.Errorf("%s: bad address %q: %w", excludedFile, parts[0], err))
			}
			excludedAddresses[key] = true
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

// loadSpecialAccountExclusions reads policy/excluded-types.txt and, for every
// pattern in it, removes the matching rows of policy/special-accounts.csv from
// the airdrop.
//
// special-accounts.csv has always been annotation that no code read. This makes
// it actionable without turning it into an address list: the decision stays
// expressed as "no exchanges" rather than as 30 hand-copied addresses that go
// stale the moment the CSV is updated.
//
// Both files are read even when no pattern is active, so a malformed CSV or a
// pattern that matches nothing is caught on every run rather than on the day
// somebody first switches an exclusion on.
func loadSpecialAccountExclusions() {
	patterns := loadExcludedTypes()

	f, err := os.Open(specialAccountsFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	// The file is hand-maintained and has been since 2022: rows have varying
	// field counts and at least one comment field contains a bare double quote.
	// Both are tolerated rather than "fixed", so that this loader never becomes a
	// reason to reformat a human-curated policy file.
	r.FieldsPerRecord = -1
	r.LazyQuotes = true

	rows, err := r.ReadAll()
	if err != nil {
		panic(fmt.Errorf("%s: %w", specialAccountsFile, err))
	}

	matched := make([]int, len(patterns))
	for i, row := range rows {
		if i == 0 || len(row) == 0 {
			continue // header
		}
		addr := strings.TrimSpace(row[0])
		if !strings.HasPrefix(addr, "cosmos1") {
			continue // blank line, or the cosmosxxxx example row
		}
		key, err := addrKey(addr)
		if err != nil {
			panic(fmt.Errorf("%s line %d: invalid address %q: %w", specialAccountsFile, i+1, addr, err))
		}

		var typ string
		if len(row) > 1 {
			typ = strings.TrimSpace(row[1])
		}
		for pi, p := range patterns {
			if p.matches(typ) {
				specialExcluded[key] = true
				matched[pi]++
			}
		}
	}

	for i, p := range patterns {
		if matched[i] == 0 {
			panic(fmt.Errorf("%s: pattern %q matched no row in %s — typo?",
				excludedTypesFile, p.raw, specialAccountsFile))
		}
		fmt.Printf("excluded-types: %q matched %d row(s)\n", p.raw, matched[i])
	}
}

// typePattern is an exact type match, or a prefix match if it ends in "*".
type typePattern struct {
	raw    string
	prefix string
	glob   bool
}

func (p typePattern) matches(typ string) bool {
	typ = strings.ToLower(strings.TrimSpace(typ))
	if p.glob {
		return strings.HasPrefix(typ, p.prefix)
	}
	return typ == p.prefix
}

func loadExcludedTypes() []typePattern {
	content := osm.MustReadFile(excludedTypesFile)

	var patterns []typePattern
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Allow a trailing "# comment" on an active line.
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" {
			continue
		}
		p := typePattern{raw: line}
		if strings.HasSuffix(line, "*") {
			p.glob = true
			p.prefix = strings.ToLower(strings.TrimSuffix(line, "*"))
		} else {
			p.prefix = strings.ToLower(line)
		}
		if strings.Contains(p.prefix, "*") {
			panic(fmt.Errorf("%s: %q — \"*\" is only allowed as the last character", excludedTypesFile, line))
		}
		patterns = append(patterns, p)
	}
	return patterns
}
