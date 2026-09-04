##
## Vesting (Constitution §132-138)
##
## OFF by default. With VESTING_START unset this Makefile behaves exactly as it
## did before the vesting pass existed, and balances.txt is byte-identical.
##
##   §132  "All Genesis $GNOT allocations vest on a common schedule: 4% unlocked
##          on the day $GNOT becomes transferrable ... and a 4% unlock every
##          subsequent month for 24 months."
##
## A continuous linear schedule over 24 months expresses that exactly, because
## 4 + 96*m/24 = 4(m+1): at month m the holder has 4(m+1)% unlocked either way.
## So OriginalVesting = 96% of the balance, start = the transferability date,
## end = start + 24 months.
##
## Turn it on by setting both times (unix seconds):
##
##   make VESTING_START=1780000000 VESTING_END=1843072000
##
## VESTING_EXEMPT is a space-separated list of addresses that receive no
## schedule. It exists for §136-138 ("150,000,000 $GNOT from the Investors
## allocation will be unlocked at the mainnet launch"), which is only
## expressible once that tranche has an address of its own.
##
## Output grammar is the one gnogenesis balances add parses
## (gno.land/pkg/gnoland/balance.go):
##
##   <addr>=<coins>;vesting=<coins>,<start>,<end>
##
VESTING_START      ?=
VESTING_END        ?=
VESTING_UNLOCK_PCT ?= 4
VESTING_EXEMPT     ?=

all: balances.txt.gz README.md

re: clean all

balances.txt.gz: balances.txt
	rm -f balances.txt.gz
	gzip -n -k balances.txt

balances.txt: non-airdrop.txt ../allocate/genbalance.txt.gz
	# generate file
	> $@.tmp
	cat non-airdrop.txt | cut -d'#' -f1 | grep -vE '^\s*$$' >> $@.tmp
	zcat ../allocate/genbalance.txt.gz | cut -d: -f2 >> $@.tmp
	cat $@.tmp  | sed 's/=/ /;s/ugnot//' | gawk '{seen[$$1] += $$2 } END {for (i in seen) printf "%s=%dugnot\n", i,seen[i]  }'   |sort -t = -k 2 -n -r > $@
	rm -f $@.tmp
	$(MAKE) vest TARGET=$@

# vest rewrites TARGET in place, adding a ;vesting= suffix to every row.
#
# Applied here rather than in allocate/ because the schedule is a function of an
# address's FINAL balance, and that is only known after the premine has been
# merged and duplicate addresses summed above.
#
# No-op unless VESTING_START is set.
vest:
ifneq ($(VESTING_START),)
	@test -n "$(VESTING_END)" || { echo "VESTING_END is required when VESTING_START is set"; exit 1; }
	gawk -v pct='$(VESTING_UNLOCK_PCT)' -v start='$(VESTING_START)' -v end='$(VESTING_END)' \
	     -v exempt='$(VESTING_EXEMPT)' ' \
	  BEGIN { \
	    n = split(exempt, xs, " "); for (i = 1; i <= n; i++) ex[xs[i]] = 1; \
	    if (pct < 0 || pct > 100) { print "VESTING_UNLOCK_PCT must be 0..100" > "/dev/stderr"; exit 1 } \
	    if (start >= end) { print "VESTING_START must be before VESTING_END" > "/dev/stderr"; exit 1 } \
	  } \
	  { \
	    eq = index($$0, "="); \
	    addr = substr($$0, 1, eq - 1); \
	    amt = substr($$0, eq + 1, length($$0) - eq - 5) + 0; \
	    if (addr in ex) { print; next } \
	    if (amt * pct >= 9007199254740992) { \
	      printf "vesting: %s=%d overflows exact integer math at pct=%d\n", addr, amt, pct > "/dev/stderr"; exit 1 \
	    } \
	    unlocked = int(amt * pct / 100); \
	    vest = amt - unlocked; \
	    if (vest <= 0) { print; next } \
	    printf "%s=%dugnot;vesting=%dugnot,%s,%s\n", addr, amt, vest, start, end; \
	  }' $(TARGET) > $(TARGET).vest
	mv $(TARGET).vest $(TARGET)
	@echo "vesting applied: $(VESTING_UNLOCK_PCT)% unlocked at $(VESTING_START), linear to $(VESTING_END)"
endif

README.md: balances.txt
	> $@.tmp
	echo "# Genesis" >> $@.tmp
	echo >> $@.tmp

	echo "## lines" >> $@.tmp
	echo '```' >> $@.tmp
	wc -l balances.txt >> $@.tmp
	echo '```' >> $@.tmp
	echo >> $@.tmp

	echo "## sum" >> $@.tmp
	echo '```' >> $@.tmp
	cat balances.txt | cut -d';' -f1 | cut -d= -f2 | sed s/ugnot// | awk '{s+=$$1} END {print s "ugnot"}' >> $@.tmp
	cat balances.txt | cut -d';' -f1 | cut -d= -f2 | sed s/ugnot// | awk '{s+=$$1} END {print int(s/1000000) "gnot"}' >> $@.tmp
	echo '```' >> $@.tmp
	echo >> $@.tmp

	echo "## duplicate accounts" >> $@.tmp
	echo '```' >> $@.tmp
	for addr in `cat balances.txt | cut -d= -f1 | sort | uniq -c  | grep -vE ' 1 ' | awk '{print $$2}'`; do grep $$addr balances.txt; done >> $@.tmp
	echo '```' >> $@.tmp
	echo >> $@.tmp

	echo "## top 100 accounts" >> $@.tmp
	echo '```' >> $@.tmp
	head -n 100 balances.txt >> $@.tmp
	echo '```' >> $@.tmp
	echo >> $@.tmp

	mv $@.tmp $@

clean:
	rm -f balances.txt
