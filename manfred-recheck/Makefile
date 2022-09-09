HEIGHT = 10562840
EXPORT = snapshot/cosmos_$(HEIGHT)_export.json
FOCUSES = auth.accounts authz.authorization bank.balances bank.supply \
	capability.owners distribution feegrant.allowances \
	gov.proposals ibc.channel_genesis ibc.client_genesis ibc.connection_genesis \
	interchainaccounts liquidity slashing staking.delegations staking.redelegations \
	staking.unbonding_delegations staking.validators transfer
JSON_FOCUSES = $(addprefix snapshot/,$(addsuffix .json,$(FOCUSES)))
SUMMARIES = summaries/account_numbers.txt summaries/uatom_holders.csv \
	summaries/uatom_sum_and_count.txt summaries/delegations.csv \
	summaries/delegations_grouped.csv summaries/delegations_sum_and_count.txt \
	summaries/undelegations.csv summaries/undelegations_grouped.csv \
	summaries/undelegations_sum_and_count.txt
EXPORTS = exports/top10000.csv

all: base summaries sums build-sqlite

##
## Sqlite
##

build-sqlite: summaries
	./build-sqlite.sh
.PHONY: build-sqlite

run-sqlite: db.sqlite
	sqlite3 $<
.PHONY: run-sqlite

$(EXPORTS):
	@make export-sqlite
export-sqlite: db.sqlite
	./export-sqlite.sh
.PHONY: compute-sqlite

db.sqlite: skip.csv
	@make build-sqlite


##
## Summaries
##

sums: shasums.txt
.PHONY: sums
shasums.txt: $(JSON_FOCUSES) $(SUMMARIES) $(EXPORTS)
	sha1sum */*.* | tee shasums.txt

summaries: $(SUMMARIES)
.PHONY: summaries

summaries/undelegations_sum_and_count.txt: summaries/undelegations_grouped.csv
	cat summaries/undelegations_grouped.csv | gawk -F',' '{ sum += $$2 } END{ printf "%d %d\n", sum, NR }' | tee $@

summaries/undelegations_grouped.csv: summaries/undelegations.csv
	cat summaries/undelegations.csv | gawk -M -F',' 'NR == 1 {next} {a[$$1] += $$2} {b[$$1] += 1} END {for (i in a) {if (a[i]>=1) {printf "%s %d %d\n", i, a[i], b[i]}}}' | sort -rnk2 | tr " " "," > $@
	wc -l $@

summaries/undelegations.csv: snapshot/staking.unbonding_delegations.json
	cat snapshot/staking.unbonding_delegations.json | jq -r '.[] | .delegator_address + "," + (.entries[].balance)' | sort > $@
	wc -l $@

summaries/delegations_sum_and_count.txt: summaries/delegations_grouped.csv
	cat summaries/delegations_grouped.csv | gawk -F',' '{ sum += $$2 } END{ printf "%d %d\n", sum, NR }' | tee $@

summaries/delegations_grouped.csv: summaries/delegations.csv
	cat summaries/delegations.csv  | gawk -M -F',' 'NR == 1 {next} {a[$$1] += $$2} {b[$$1] += 1} END {for (i in a) {if (a[i]>=1) {printf "%s %d %d\n", i, a[i], b[i]}}}' | sort -rnk2 | tr " " "," > $@
	wc -l $@

summaries/delegations.csv: snapshot/staking.delegations.json
	cat snapshot/staking.delegations.json  | jq -r '.[] | .delegator_address + "," + .shares' | sort > $@
	wc -l $@

summaries/uatom_sum_and_count.txt: summaries/uatom_holders.csv
	cat summaries/uatom_holders.csv | gawk -F',' '{ sum += $$2 } END{ print sum, NR }' | tee $@
	wc -l $@

summaries/uatom_holders.csv: snapshot/bank.balances.json
	cat snapshot/bank.balances.json  | jq -r '.[] | select(.coins[].denom == "uatom") | .address + " " + (.coins[] | select(.denom == "uatom") | .amount)' | sort -rnk2 | tr " " "," > $@
	wc -l $@

summaries/account_numbers.txt: snapshot/auth.accounts.json
	cat snapshot/auth.accounts.json | jq -r '.[].account_number' | sort -n > $@
	wc -l $@

##
## Snapshot extracts
##

base: $(JSON_FOCUSES)
.PHONY: base

$(JSON_FOCUSES): $(EXPORT)
	cat $< | jq .app_state.$(@:snapshot/%.json=%) > $@

##
## Raw Snapshot
##

$(EXPORT):
	@echo "No such file: $(EXPORT)."
	@echo
	@echo "You need to either build it (make gen-export) or download it (make dl-export)."
	@exit 1

gen-export:
	gaiad export --height=$(HEIGHT) 2>&1 | jq . | tee $(EXPORT)

dl-export:
	wget -O $(EXPORT) https://test1.gno.land/static/cosmos_10562840_export.json
