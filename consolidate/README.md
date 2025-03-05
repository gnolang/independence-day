# Snapshot consolidation

Height 10562840 on 5/20/2022 8:00 PDT

[consolidated snapshot](./snapshot_consolidated_10562840.json.gz) 27MB

   * Address: atom holder with balances or delegations
   * []Coin: all assets in the account.
   * Coin "duatom" is the delegated uatom. The amount is the entire delegation converted from shares back to uatoms.
   * Vote: the last voting option for proposal 69


# About the data


## Where the data is from ?

* Balance and delegation data are from the 5/20 export data. If you want to capture the snapshot yourself, follow the steps in [README.md](../README.md)


* In this [directory](./), you will find the last voting submission of proposal 69 (last_vote_pro69.json.zip ) and validator states with token and share ratio (validators.json). Those were received from the archive node database downloaded from  quicksync.io cosmos hub 4 (https://quicksync.io/networks/cosmos.html)

## How data was consolidated ?


you can find detailed steps from this [repo](https://github.com/piux2/gnobounty7)

    /build/exactor  merge --b balances.json, --d delegations.json --val validators.json --vote last_vote_pro69.json > snapshot_consolidated.json

balances.json

	  jq '.app_state.bank.balances' cosmos_10562840_export.json > balances.json

delegations.json

     jq '.app_state.staking.delegations'  cosmos_10562840_export.json > delegations.json

validators.json:

   [instruction](https://github.com/piux2/gnobounty7/blob/main/README3.md#export-validator-token-and-shares-information-from-postgresql)

last_vote_pro69.json:

   [instruction](https://github.com/piux2/gnobounty7/blob/main/README3.md#export--last_vote_pro69-as-json)

#### Notes:

* duatom amount delegations shares are retrieved from multiple validators according to each validator's token share ratio, a.k.a exchange rates. Some validators got slashed in the past, they the token share ratio less than 1.
* balances.json does not contain addresses with zero balances even if there are delegations from the same address. We recreated accounts and balances if those addresses have delegations at the snapshot height.
* last_vote_pro69.json includes all votes submitted when proposal 69 was active. When SDK tallies the voting power on votes, it only includes votes casted through the bonded validators. However, we includes all votes as long as they are valid submissions.

## How to verify ?

* option1: recreate the result from the [repo](https://github.com/piux2/gnobounty7) and see if there are steps and data was not correct
* option2: Use other methods to cross-check the final data set provided.