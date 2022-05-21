#!/bin/sh

# query txs with vote on prop 69
#mkdir -p votes
#for page in `seq 1 1100`; do
#    $GAIAD_BIN query txs --events=proposal_vote.proposal_id=69 --limit=100 --page=$page --output=json > votes/$page.json
#done

# extract txs' votes in csv
#cat votes/*.json | jq -r '.txs[] | (.timestamp + "," + (.tx.body.messages[] | (.voter + "," + .option)))' | sort -k 1,1 | grep -v ',,' > votes.csv

# get latest vote per unique voter
#while IFS="," read -r a b c; do printf "%s,%s,%s,%d\n" "$a" "$b" "$c" $(date -d"$a" +"%s"); done < votes.csv | \
#    awk 'BEGIN{FS=OFS=","} {it=$NF; NF--
#    	 if (max[$2]<it) {max[$2]=it; res[$2]=$0}}
#         END {for (i in max) print res[i]}' | sort -k 1,1 > votes-unique.csv
