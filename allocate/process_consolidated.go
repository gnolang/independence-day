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
	publicSaleFile = "../mkgenesis/publicsale.txt"
	investorsFile  = "../mkgenesis/investors.txt"
	balancesFile   = "../mkgenesis/balances.txt.gz"
)

// TOTAL_SUPPLY is the finalized cap. It covers everything that ends up in
// mkgenesis/balances.txt.gz — the buckets below AND the non-airdrop premine.
const TOTAL_SUPPLY = 1333000000

// TOTAL_PREMINE_NON_AIRDROP is the sum of mkgenesis/non-airdrop.txt: three named
// contributors, 45 GitHub requesters, 14 multisig signer gas floats and the
// examples/ contributor airdrop. The finalized 7-bucket breakdown does not
// budget for it, so somebody has to pay for it.
//
// Was 2455000 until the test1 and test2 rows were removed on 2026-09-03 — both
// were funded from mnemonics published in gnolang/gno's test fixtures. Was
// 2345000 until 14 multisig signer gas-float rows were added (moul/gno-meta#107).
// Was 2359000 until 13 examples/ package authors were added at 10,000 GNOT each
// (+130000, the contributor airdrop; gnolang/multisigs#43). That set is
// INCOMPLETE: 14 more qualifying authors have no key in any registry yet, so
// expect a further +140000 once multisigs#43 collects them. Was 2489000 until
// faucet0 and faucet1 were removed on 2026-09-09 (-2000000): there is no mainnet
// faucet, and a faucet was never an Ecosystem Treasury purpose under §322-328.
//
// TestPremineMatchesFile asserts this constant against the actual file.
const TOTAL_PREMINE_NON_AIRDROP = 489000

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

	// Six eligible founders x 1,000 GNOT, not seven: Jae is skipped, see
	// govdaoFoundersSkipped and moul/gno-meta#102. Because the founders
	// allocation is charged to the Core Treasury and Core is a fixed 40,000,000,
	// the 1,000 GNOT he does not receive simply stays in Core -- there is nothing
	// to redistribute and no absorption constant is needed.
	// TestFoundersBudgetMatchesSkipList keeps this in step with the skip list.
	TOTAL_AIRDROP_GOVDAO_FOUNDERS = 6000

	// --- The founding validator set: a gas float, not a reward -------------
	//
	// gnolang/gno's mainnet builder (misc/deployments/mainnet.gno.land) seeds
	// four founding validators, each as a (signing address, operator address)
	// pair. `gnogenesis fork valoper-seed` REQUIRES the two to be distinct, so
	// that compromising a signing key does not collapse into control of the
	// validator slot. That makes eight addresses, not four.
	//
	// Every management action on a validator is a PAID transaction: rotating the
	// signing key, editing the valoper profile in r/gnops/valopers, signalling
	// opt-out through r/sys/validators. There is no mainnet faucet, and §126
	// locks ugnot transfers to the §127 exemption list -- so an address that
	// lands at zero can never be funded afterwards and can never act. Six of the
	// eight held nothing at all; see gnolang/gno's INITIAL_VALSET_OPERATORS
	// TODO(mainnet), which is what this closes.
	//
	// 1,000 GNOT each, the same tier as the founders grant above and the multisig
	// signer float in mkgenesis/non-airdrop.txt. It is a float so a genesis
	// validator can ACT, not a payment for validating -- block rewards are not a
	// genesis matter.
	//
	// TestGenesisValidatorFloatMatchesSkipList keeps the budget in step with the
	// skip list; TestGenesisValidatorsAreFunded asserts the end state on the
	// shipped sheet, including the two funded from elsewhere.
	GENESIS_VALIDATOR_FLOAT       = 1000
	TOTAL_GENESIS_VALIDATOR_FLOAT = 6 * GENESIS_VALIDATOR_FLOAT

	// --- Chain-service addresses: a bigger float, and why ------------------
	//
	// Some addresses have to ACT on the chain from day one without being a
	// validator, a founder or a contributor. The first is the approvals oracle
	// for the inert-package policy: under `code_submission_policy=inert` a
	// post-genesis package submission is PARKED until an address in
	// vm.params pkg_approvers approves it, and approving is a paid tx. An
	// oracle that cannot pay is an oracle that approves nothing, and under §126
	// with no faucet it can never be funded afterwards -- so every
	// post-genesis submission would park forever.
	//
	// The second is the [govdao] 4-of-7 multisig, which gnolang/gno hardcodes
	// as the OWNER of realms that ship in the genesis set. Funding it is not a
	// governance grant -- GovDAO voting is paid by whoever proposes -- it is
	// the operating float for realms whose owner cannot be changed without a
	// realm upgrade. See chainServices for which realms and why r/sys/names is
	// NOT the reason.
	//
	// 5,000 GNOT, deliberately FIVE TIMES the 1,000 tier the founders grant and
	// the validator float sit at, because it is a different kind of spender.
	// Those two pay for occasional management actions -- rotate a key, edit a
	// profile, vote. A service spends per EVENT, continuously, for as long as
	// the chain accepts packages, and nobody tops it up in between: §126 leaves
	// only the exemption-listed funds able to send, so the next refill waits on
	// the transfer lock lifting or a GovDAO proposal.
	//
	// For the oracle the number is checkable rather than conventional.
	// gnolang/gno contribs/gpao -- the package-approver daemon that would hold
	// this key -- broadcasts one MsgEnablePackage per approval at a default gas
	// fee of 1,000,000 ugnot, so 5,000 GNOT is ~5,000 approvals. Its own
	// per-run max-spend bound is 100 GNOT, now a FIFTIETH of the float rather
	// than a tenth, so a misbehaving run is even further from draining it.
	//
	// The multisig gets the same tier for the same shape of reason: it owns two
	// realms rather than one daemon, its spend is episodic rather than per
	// event, and the window it has to cover is the whole transfer lock -- the
	// pinned §132 schedules run to 2028-09-11, so "until §126 lifts" is years,
	// not weeks. At the same 1,000,000 ugnot default that is ~5,000 owner
	// actions across both realms.
	//
	// Still a float, not a standing budget: topping it up is GovDAO's business
	// once §126 lifts.
	//
	// See chainServices for the list and SERVICES_CHARGED_TO_CORE for the
	// bucket. TestChainServicesAreFunded asserts the end state on the shipped
	// sheet.
	CHAIN_SERVICE_FLOAT       = 5000
	TOTAL_CHAIN_SERVICE_FLOAT = 2 * CHAIN_SERVICE_FLOAT

	// --- Constitution §120-122: three treasuries, three addresses -----------
	//
	// These used to be one undifferentiated TOTAL_AIRDROP_CONTRIBS line paid to
	// one GovDAO T1 multisig:
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

	// §333: "Present GovDAO members are not eligible for any allocation from the
	// Ecosystem Treasury genesis allocation." All seven founders are GovDAO
	// founders, so the founders allocation is charged to CORE. While the three
	// treasuries shared one line this could not be shown either way; now it can.
	FOUNDERS_CHARGED_TO_CORE = TOTAL_AIRDROP_GOVDAO_FOUNDERS

	// §120's Core Treasury is for "Core Software + Essential Services", which
	// is a positive match for the chain-service floats rather than an argument
	// by elimination: an oracle that gates code submission on the production
	// chain is an essential service of the core software. The other two buckets
	// exclude it outright -- §122 is "validation services only" and an approvals
	// oracle is not a validator, and §121 is "External Contributions" with §346
	// restricting it to "contributors whose real human identity is known and
	// recorded", which is the clause that removed the faucet.
	SERVICES_CHARGED_TO_CORE = TOTAL_CHAIN_SERVICE_FLOAT

	// §121 makes the Ecosystem Treasury "for prior and future Gno.land ecosystem
	// development" and §140 makes GovDAO responsible for distributing it "to
	// prior and future Gno.land ecosystem contributors" — so the 2022 contributor
	// premine is charged to ECOSYSTEM. Tracking PREMINE_ABSORBED_FROM_CONTRIBS
	// rather than TOTAL_PREMINE_NON_AIRDROP keeps the option A/B/C switch above
	// working: under option A nothing is charged to any treasury.
	//
	// The 2,000,000 GNOT of faucet funding that used to sit in here — chain
	// operations rather than ecosystem development, and not a purpose §322-328
	// can accommodate — was removed on 2026-09-09. There is no mainnet faucet.
	PREMINE_CHARGED_TO_ECOSYSTEM = PREMINE_ABSORBED_FROM_CONTRIBS

	// §122 names the Validator Services Treasury, and a gas float that exists
	// solely so a founding validator can rotate its key, edit its profile or
	// opt out IS a validator service -- so the float is charged THERE. The two
	// alternatives both fail §226's purpose test: §346 restricts Ecosystem to
	// "contributors whose real human identity is known and recorded", which is
	// the clause that removed the faucet, and Core already carries the founders.
	VALIDATOR_FLOAT_CHARGED_TO_VALIDATOR = TOTAL_GENESIS_VALIDATOR_FLOAT

	// Net amounts written to the three treasury addresses.
	TOTAL_TREASURY_CORE_NET      = TOTAL_TREASURY_CORE - FOUNDERS_CHARGED_TO_CORE - SERVICES_CHARGED_TO_CORE
	TOTAL_TREASURY_ECOSYSTEM_NET = TOTAL_TREASURY_ECOSYSTEM - PREMINE_CHARGED_TO_ECOSYSTEM
	TOTAL_TREASURY_VALIDATOR_NET = TOTAL_TREASURY_VALIDATOR - VALIDATOR_FLOAT_CHARGED_TO_VALIDATOR

	// --- Constitution §123-124 + §136-138: Investors and NT,LLC -------------
	//
	// §123 Investors 300,000,000 and §124 NT,LLC 332,000,000 are two buckets,
	// not one; they used to be a single 632,000,000 line paid to nt1. §136 then
	// carves the Investors bucket in two:
	//
	//   §136  "150,000,000 $GNOT from the Investors allocation will be unlocked
	//          at the mainnet launch"
	//
	// One address carries one vesting schedule, so the unlocked tranche needs an
	// address of its own. While all 632M sits on nt1 the exception is not merely
	// unimplemented — it is UNEXPRESSIBLE.
	TOTAL_INVESTORS_UNLOCKED = 150000000 // liquid at mainnet, §136
	TOTAL_INVESTORS_VESTING  = 150000000 // §132 schedule
	TOTAL_AIRDROP_NT         = TOTAL_INVESTORS_UNLOCKED + TOTAL_INVESTORS_VESTING
	TOTAL_AIRDROP_NT_LLC     = 332000000

	// --- The public token sale ---------------------------------------------
	//
	// The Sonar sale (Ethereum mainnet, 0x959f2ceE7B6C2095d228692eCb2E4744f2D3fDb4)
	// settled 122 wallets for 21,604,687.430103 GNOT. Buyers are investors, and
	// their tokens are contractually lockup-free — so the sale is not an eighth
	// bucket, it is the part of the §136 tranche that is already spoken for by
	// name. Total supply does not move; INVESTORS_UNLOCKED_ADDRESS holds the
	// tranche NET of it and the rest is written to the 79 rows in
	// mkgenesis/publicsale.txt.
	//
	// This one is in ugnot, not GNOT, and cannot be anything else: a sale
	// allocation is USD/clearing-price and lands on an arbitrary ugnot value.
	// Hence assignUgnot() beside assign(). TestPublicSaleMatchesFile asserts the
	// constant against the actual file.
	TOTAL_PUBLIC_SALE_UGNOT = 21604687430103 // 21,604,687.430103 GNOT

	// Investor and partner distributions already owed to named counterparties —
	// market making, exchange listing and integration, ecosystem liquidity — paid
	// at genesis out of the same §136 tranche and by the same reasoning as the
	// sale: not an eighth bucket, just the part of the tranche already spoken
	// for. 9 rows in mkgenesis/investors.txt; the counterparties are not named
	// there and the mapping is held privately.
	//
	// Obligations settled AFTER genesis are deliberately absent, because the
	// tranche multisig can pay them later and a genesis row cannot be undone.
	// TestInvestorDistributionsMatchFile asserts this against the file.
	TOTAL_INVESTOR_DISTRIBUTIONS_UGNOT = 16076470000000 // 16,076,470 GNOT

	// What actually reaches INVESTORS_UNLOCKED_ADDRESS: 112,318,842.569897 GNOT.
	// The three §136 lines must still sum to 150,000,000 — TestSplitPreservesTheAggregate.
	TOTAL_INVESTORS_UNLOCKED_UGNOT = TOTAL_INVESTORS_UNLOCKED*1000000 -
		TOTAL_PUBLIC_SALE_UGNOT -
		TOTAL_INVESTOR_DISTRIBUTIONS_UGNOT

	// The six real multisigs, from gnolang/multisigs config.toml. Each is the
	// SAME signer set as the account it succeeds -- the three treasuries are the
	// [govdao] members, the three ex-nt1 buckets are the [nt1] members -- with
	// one provably-unspendable "salt" key added so that six purposes get six
	// distinct addresses instead of collapsing onto two.
	//
	// A multisig address is the hash of its (threshold, members) tuple, so the
	// same people at the same threshold always produce the same address. The salt
	// key is a real secp256k1 point whose x coordinate is a small integer counted
	// up from 1, so no private key for it can exist and it can never contribute a
	// signature. Effective thresholds are therefore unchanged: 4-of-7 for the
	// treasuries, 4-of-6 for the ex-nt1 buckets.
	//
	// Neither g1pxj9x5... (nt1) nor g1sze988... (GovDAO T1) is reused. Reuse was
	// available for one successor each, but a reused address carries the old
	// account's history and makes "which bucket is this?" unanswerable from the
	// address alone -- which is the whole point of the split.
	TREASURY_CORE_ADDRESS      = "g1shmvjxkvx9kgnrta5rzwcpdqszy4pkfvv9qjz9" // §120
	TREASURY_ECOSYSTEM_ADDRESS = "g1ugke9x9ylrlex0lxcgw7eu0mdftvcrgglru0l0" // §121
	TREASURY_VALIDATOR_ADDRESS = "g1kj5ag4xdjws00rfzg49x6lljv34pty5uchcq2p" // §122
	INVESTORS_UNLOCKED_ADDRESS = "g1j3et7juxr3npgdll3lml3mpv0y6m49rztjnf76" // §136, no vesting schedule
	INVESTORS_VESTING_ADDRESS  = "g1x7tm26g9wj84cmg3cs74uwf3g9lqj4mjp6gax3" // §132
	NT_LLC_ADDRESS             = "g1pku9u3jwr8k8vjpfypqzd0uhmrwr35zk0f8u7p" // §124

	MULTISIG_NT2_ADDRESS = "g1sp27hn785v3kud6cg9dnhrng7wzp9cnljffhcg" //nt2: special case handling for aib accounts
)

var ibcEscrowAddress = map[string]bool{}
var excludedAddresses = map[string]bool{}

// specialExcluded holds addresses removed by CLASS via policy/excluded-types.txt,
// keyed by the canonical g1 form so one entry covers every bech32 encoding of the
// same key. Empty unless a pattern is uncommented in that file.
var specialExcluded = map[string]bool{}

func init() {
	validateHardcodedAddresses()
	loadEscrowAddress()
	loadExcludedAddresses()
	loadSpecialAccountExclusions()
}

func main() {
	// `go run . readme` regenerates README.md from the committed
	// genbalance.txt.gz. It is a separate subcommand rather than a step at the
	// end of the run because recomputing the allocation takes ~90s, and the
	// report only ever describes the artifact that is already on disk.
	if len(os.Args) > 1 && os.Args[1] == "readme" {
		if err := runReadme(os.Args[2:]); err != nil {
			panic(err)
		}
		return
	}

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

	// Allocate Investors and NT,LLC to separate addresses (§123-124), with the
	// §136 mainnet-unlocked tranche separated from the vesting remainder.
	//
	// The unlocked tranche is written NET of the public sale: the sale is the
	// part of it that is already owed to named buyers, and those buyers are paid
	// directly by mkgenesis/publicsale.txt. Supply is unchanged either way.
	assignUgnot(totalDist, INVESTORS_UNLOCKED_ADDRESS, TOTAL_INVESTORS_UNLOCKED_UGNOT)
	assign(totalDist, INVESTORS_VESTING_ADDRESS, TOTAL_INVESTORS_VESTING)
	assign(totalDist, NT_LLC_ADDRESS, TOTAL_AIRDROP_NT_LLC)

	// Allocate GovDAO founders budget (1000 GNOT each), skipping the founders
	// who already hold a snapshot entitlement. The divisor is the number of
	// ELIGIBLE founders, not len(govdaoFounders) -- otherwise skipping one would
	// silently change everybody else's amount.
	eligibleFounders := 0
	for _, addr := range govdaoFounders {
		if _, skipped := govdaoFoundersSkipped[addr]; !skipped {
			eligibleFounders++
		}
	}
	for _, addr := range govdaoFounders {
		if reason, skipped := govdaoFoundersSkipped[addr]; skipped {
			fmt.Printf("skipping govdao founders allocation for %s: %s\n", addr, reason)
			continue
		}
		assign(totalDist, addr, TOTAL_AIRDROP_GOVDAO_FOUNDERS/eligibleFounders)
	}

	// Allocate the founding validator set's gas float (§122). Unlike the founders
	// budget this is a flat per-address amount rather than a budget divided by the
	// eligible count: skipping an address that is already funded must leave every
	// other slot at exactly 1,000 GNOT, and the skipped 1,000 simply stays in the
	// Validator Services Treasury -- the same shape as Jae's skipped founders
	// grant staying in Core.
	for _, addr := range genesisValidators {
		if reason, skipped := genesisValidatorsSkipped[addr]; skipped {
			fmt.Printf("skipping genesis validator float for %s: %s\n", addr, reason)
			continue
		}
		assign(totalDist, addr, GENESIS_VALIDATOR_FLOAT)
	}

	// Allocate the chain-service floats (§120). Flat per-address, same as above.
	for _, svc := range chainServices {
		fmt.Printf("chain service float for %s: %s\n", svc.addr, svc.role)
		assign(totalDist, svc.addr, CHAIN_SERVICE_FLOAT)
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

// chainService is an address that must be able to send a transaction from block
// one in order for some part of the chain to function, without being a
// validator, a founder or a contributor. The role is carried alongside the
// address because "why does this address hold money?" must be answerable from
// this file alone -- a bare g1 in a list is exactly the kind of row a reviewer
// cannot check.
type chainService struct {
	addr string
	role string
}

// chainServices is that list. Each entry gets CHAIN_SERVICE_FLOAT, charged to
// the §120 Core Treasury (see SERVICES_CHARGED_TO_CORE).
//
// There is no skip list here, unlike genesisValidators: none of these addresses
// holds anything from another line today, and if one ever does, assign() panics
// and asks for a decision rather than silently summing or overwriting.
//
// The [govdao] 4-of-7 multisig was previously excluded here on the grounds that
// a governance body is not a service and Core may not be its bucket. That was
// reasoning from the wrong capability. The reason given was r/sys/names, and
// r/sys/names is the one realm where the objection holds: its `admin` gates
// Enable() and NOTHING else, Enable is a one-shot genesis call, and the realm's
// own source says the address is "dead weight" afterwards -- pause/unpause runs
// through a GovDAO T1 proposal (ProposeSetPaused), not through this key. Funding
// it for names administration would indeed have been funding nothing.
//
// But the same address is the hardcoded OWNER of two other realms that ship in
// the mainnet genesis set:
//
//	r/gnoland/blog          adminAddr      -- post, edit, moderate the official blog
//	r/gnoland/boards2/v1    gPerms         -- realm permissions owner
//
// (Confirmed against gnolang/gno master with `gno tool deplist -test-dep` over
// the mainnet FILTERED_PACKAGES. r/gnoland/home and r/demo/defi/foo20 hardcode
// it too but do NOT reach genesis, so they are not part of this.)
//
// Those are ordinary paid txs, forever, and the owner is hardcoded at realm
// source -- it cannot be reassigned without a realm upgrade. An owner that
// lands at zero under §126 with no faucet cannot post to the chain's own blog
// or administer its own boards, and cannot be topped up until the transfer lock
// lifts. So this is a service float for realm operation, not a governance
// grant: GovDAO voting is paid by whoever proposes, and that is unaffected.
var chainServices = []chainService{
	{
		addr: "g1yaaa6rcp4ew5yjzdj4yms596wx2dtrj3a86704",
		role: "inert-package approvals oracle (vm.params pkg_approvers)",
	},
	{
		addr: "g1skl80cuz8zq3lul9pgz5pc35l2pfzgxgfpsqkx",
		role: "[govdao] 4-of-7 multisig — hardcoded owner of r/gnoland/blog and r/gnoland/boards2/v1",
	},
}

// genesisValidators is the founding validator set of gnoland-1, as
// (signing address, operator address) pairs, transcribed from INITIAL_VALSET
// and INITIAL_VALSET_OPERATORS in gnolang/gno's
// misc/deployments/mainnet.gno.land/gen-genesis.sh.
//
// Both halves of each pair are here on purpose. The operator is the management
// plane — it rotates the signing key, edits the valoper profile and signals
// opt-out — and is the address that would otherwise land at zero and be stuck
// there. The signing address is funded too, at the same tier, so that a
// validator slot is never inert for want of gas on whichever of its two keys is
// at hand; `fork valoper-seed` forces the two to be distinct precisely because
// they are held differently (the signing key lives in tmkms/horcrux/an HSM), so
// funding only one of them leaves a hole that §126 makes permanent.
//
// The four power-60 slots are Gnocore, OnBloc, Samourai Crew and Berty. This
// list is a MIRROR of gnolang/gno's: if the valset changes there, it changes
// here, and the gas float follows the set that actually launches the chain.
var genesisValidators = []string{
	"g1mmgvcssjw6x4fzphupfg6mtxqt36v000c5rf2a", // gno-core-validator-1 (signing)
	"g1aeddlftlfk27ret5rf750d7w5dume3kcsm8r8m", // gno-core-validator-1 operator (aeddi)
	"g1hqhetnnz0raw5hps6yxexl7q09a6f8w3anlptt", // onbloc-validator-1 (signing)
	"g12gtvlcexzgax49nvvkvhp2u0v6eejhunq0074p", // onbloc-validator-1 operator
	"g15t7f9q6km3ldt885duwl8xu5dncs98528amk4f", // samourai-crew-validator-1 (signing)
	"g1n9y62agq998jt8w59az60xcqlftjknjg2grhn4", // samourai-crew-validator-1 operator
	"g1l983yy3kpmapyzcfy53y5charfxupa5czjalea", // berty-validator-1 (signing)
	"g1qynsu9dwj9lq0m5fkje7jh6qy3md80ztqnshhm", // berty-validator-1 operator
}

// genesisValidatorsSkipped lists the genesis validator addresses that do NOT
// receive the §122 gas float, because they are already funded to at least
// GENESIS_VALIDATOR_FLOAT from another line in this genesis. Funding them again
// would sum (aeddi would panic in assign(); the Berty operator would quietly
// become 2,000 GNOT via mkgenesis's accumulate()) and would make "why is this
// one different?" unanswerable from the sheet.
//
// The rule is one line: skip a genesis validator address that already holds at
// least 1,000 GNOT, and leave the 1,000 it does not receive in the Validator
// Services Treasury — exactly what Jae's skipped founders grant does in Core.
// TestGenesisValidatorsAreFunded asserts the END STATE for all eight addresses
// on the shipped sheet, so a skip whose other funding line disappears fails
// here rather than at launch.
var genesisValidatorsSkipped = map[string]string{
	"g1aeddlftlfk27ret5rf750d7w5dume3kcsm8r8m": "gno-core operator (aeddi) - already holds the 1,000 GNOT govdaoFounders grant",
	"g1qynsu9dwj9lq0m5fkje7jh6qy3md80ztqnshhm": "berty operator - already holds the 1,000 GNOT multisig signer float in mkgenesis/non-airdrop.txt",
}

var govdaoFounders = []string{
	"g1ecsuj0q572jr0dhu29q9njtnmw03hyu7tyyvv6", // Jae
	"g1manfred47kzduec920z88wfr64ylksmdcedlf5", // Manfred
	"g1gzhj234kpajz963z5vf42j4ylddscnkez2wvly", // Dongowon
	"g1m0rgan0rla00ygmdmp55f5m0unvsvknluyg2a4", // Morgan
	"g127l4gkhk0emwsx5tmxe96sp86c05h8vg5tufzq", // Maxwell
	"g1e6gxg5tvc55mwsn7t7dymmlasratv7mkv0rap2", // Milos
	"g1aeddlftlfk27ret5rf750d7w5dume3kcsm8r8m", // Aeddi
}

// govdaoFoundersSkipped lists founders who do NOT receive the fixed 1,000 GNOT
// founders allocation, because they already hold a snapshot entitlement that
// assign() rightly refuses to overwrite.
//
// gnolang/independence-day#62 repointed Jae's entry onto
// g1ecsuj0q572jr0dhu29q9njtnmw03hyu7tyyvv6, which is a top-10 Cosmos Hub
// recipient holding 7,545,247.650036 GNOT. The old address held nothing, which
// is why this never fired before. assign() panicked and asked for a decision;
// the decision is recorded in moul/gno-meta#102: the snapshot entitlement
// stands, and the founders allocation is skipped for him. His genesis balance is
// therefore 7,545,247.650036 GNOT, not 7,545,248.650036 and not 1,000.
//
// Every entry here must be an address that also appears in govdaoFounders, and
// the count must be consistent with TOTAL_AIRDROP_GOVDAO_FOUNDERS. Both are asserted.
var govdaoFoundersSkipped = map[string]string{
	"g1ecsuj0q572jr0dhu29q9njtnmw03hyu7tyyvv6": "Jae - holds a Cosmos Hub snapshot entitlement (moul/gno-meta#102)",
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
	assignUgnot(dist, addr, int64(gnot)*1000000)
}

// assignUgnot is assign() at ugnot precision, for an allocation that is not a
// whole number of GNOT. The public sale is the only one: it is denominated in
// USD at a clearing price, so its residue after division is arbitrary.
func assignUgnot(dist map[string]Distribution, addr string, ugnot int64) {
	if existing, ok := dist[addr]; ok && !existing.Ugnot.IsZero() {
		panic(fmt.Errorf(
			"refusing to overwrite an existing entitlement: %s already holds %s ugnot "+
				"(from source address %s) and would be replaced by a fixed allocation of %d ugnot; "+
				"decide explicitly whether the two should be summed",
			addr, whole(existing.Ugnot.String()), existing.Account.Address, ugnot))
	}

	dist[addr] = Distribution{
		Account:    Account{Address: addr},
		GnoAddress: addr,
		Ugnot:      types.NewDec(ugnot),
	}
}

// validateHardcodedAddresses checks every address this program writes into the
// balance sheet without deriving it from a snapshot. A malformed one would
// otherwise be discovered by whatever consumes the output — or, worse, not be
// discovered, since nothing downstream asserts the address format.
func validateHardcodedAddresses() {
	seen := make(map[string]string, len(govdaoFounders)+len(genesisValidators)+7)

	// format checks the address itself. Used alone for entries that are listed
	// but not paid — a skipped genesis validator is funded by another role, and
	// that role legitimately owns the address.
	format := func(addr, role string) {
		if _, err := addrKey(addr); err != nil {
			panic(fmt.Errorf("%s: invalid address %q: %w", role, addr, err))
		}
	}

	// check is format plus "no address is paid twice". Two fixed allocations on
	// one address is always a mistake: assign() would panic if both landed in the
	// distribution, and where it would not (a row added in mkgenesis) the two
	// would silently sum.
	check := func(addr, role string) {
		format(addr, role)
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

	// The validator set is its own namespace first — a duplicated slot there is a
	// transcription error in the mirrored valset, and it must be caught even when
	// both copies are skipped.
	valseen := make(map[string]string, len(genesisValidators))
	for i, addr := range genesisValidators {
		role := fmt.Sprintf("genesisValidators[%d]", i)
		format(addr, role)
		if prev, dup := valseen[addr]; dup {
			panic(fmt.Errorf("address %s appears twice in the genesis valset, as %s and %s", addr, prev, role))
		}
		valseen[addr] = role

		if _, skipped := genesisValidatorsSkipped[addr]; skipped {
			continue // paid by another role, which already owns the slot in `seen`
		}
		check(addr, role)
	}
	for addr := range genesisValidatorsSkipped {
		if _, ok := valseen[addr]; !ok {
			panic(fmt.Errorf("genesisValidatorsSkipped lists %s, which is not in genesisValidators", addr))
		}
	}

	for i, svc := range chainServices {
		if svc.role == "" {
			panic(fmt.Errorf("chainServices[%d] (%s) has no role — an unexplained funded address is not reviewable", i, svc.addr))
		}
		check(svc.addr, fmt.Sprintf("chainServices[%d] (%s)", i, svc.role))
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
	// Skip excluded addresses.
	//
	// Matching is on the 20-byte payload, NOT the bech32 string. Every
	// excluded.txt entry is written in cosmos1… form, and qualifyAtone() passes
	// the atone1… encoding of the same keys — so a string comparison excluded
	// them on the Cosmos side and silently let them through on the AtomOne side.
	if key, err := addrKey(address); err == nil && excludedAddresses[key] {
		return true
	}

	// skip addresses excluded by class via policy/excluded-types.txt
	if len(specialExcluded) > 0 {
		if key, err := addrKey(address); err == nil && specialExcluded[key] {
			return true
		}
	}

	// Skip IBC transfer escrow accounts.
	//
	// These are derived, not curated: sha256("ics20-1" || 0x00 ||
	// "transfer/channel-N")[:20]. There is no preimage anyone holds, so no
	// private key exists for them on any chain. GNOT credited to one can never be
	// moved by anybody — crediting them is a burn, not an allocation.
	//
	// Matched on the 20-byte payload: the file's first column is cosmos1…, and
	// the same escrow accounts appear in the AtomOne snapshot under atone1….
	if key, err := addrKey(address); err == nil && ibcEscrowAddress[key] {
		return true
	}

	return false
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
