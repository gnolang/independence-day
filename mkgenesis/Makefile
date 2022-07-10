all: balances.txt.gz README.md

re: clean all

balances.txt.gz: balances.txt
	rm -f balances.txt.gz
	gzip -k balances.txt

balances.txt: non-airdrop.txt ../consolidate/genbalance.txt.gz
	# generate file
	> $@.tmp
	cat non-airdrop.txt | cut -d'#' -f1 | grep -vE '^\s*$$' >> $@.tmp
	zcat ../consolidate/genbalance.txt.gz | cut -d: -f2 >> $@.tmp
	cat $@.tmp  | sort -t = -k 2 -n -r > $@
	rm -f $@.tmp

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
	cat balances.txt | cut -d= -f2 | sed s/ugnot// | awk '{s+=$$1} END {print s "ugnot"}' >> $@.tmp
	cat balances.txt | cut -d= -f2 | sed s/ugnot// | awk '{s+=$$1} END {print int(s/1000000) "gnot"}' >> $@.tmp
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
