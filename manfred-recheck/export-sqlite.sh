#!/bin/sh

mkdir -p exports
sqlite3 db.sqlite > exports/top10000.csv <<EOF
.headers on
.mode csv
SELECT * FROM accounts ORDER BY cummulative_atoms DESC LIMIT 10000;
EOF
