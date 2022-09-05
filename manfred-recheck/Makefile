HEIGHT = 10562840
EXPORT = snapshot/cosmos_$(HEIGHT)_export.json
FOCUSES = auth.accounts authz.authorization bank.balances bank.supply \
	capability.owners distribution feegrant.allowances \
	gov.proposals ibc.channel_genesis ibc.client_genesis ibc.connection_genesis \
	interchainaccounts liquidity slashing staking transfer
JSON_FOCUSES = $(addprefix snapshot/,$(addsuffix .json,$(FOCUSES)))
SUMMARIES = summaries/account_numbers.txt summaries/uatom_holders.csv summaries/uatom_sum_and_count.txt

all: base summaries sums

##
## Summaries
##

sums: shasums.txt
.PHONY: sums
shasums.txt: $(JSON_FOCUSES) $(SUMMARIES)
	sha1sum */*.* | tee shasums.txt

summaries: $(SUMMARIES)
.PHONY: summaries

summaries/uatom_sum_and_count.txt: summaries/uatom_holders.csv
	cat summaries/uatom_holders.csv | gawk -F',' '{ sum += $$2 } END{ print sum, NR }' | tee $@

summaries/uatom_holders.csv: snapshot/bank.balances.json
	cat snapshot/bank.balances.json  | jq -r '.[] | select(.coins[].denom == "uatom") | .address + " " + (.coins[] | select(.denom == "uatom") | .amount)' | sort -rnk2 | tr " " "," > $@

summaries/account_numbers.txt: snapshot/auth.accounts.json
	cat snapshot/auth.accounts.json | jq -r '.[].account_number' | sort -n > $@

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
