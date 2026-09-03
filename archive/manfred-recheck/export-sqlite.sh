#!/bin/sh

mkdir -p exports

sqlite3 db.sqlite > exports/top10000.csv <<EOF
.headers on
.mode csv
SELECT * FROM accounts ORDER BY cummulative_atoms DESC LIMIT 10000;
EOF

sqlite3 db.sqlite > exports/top1000.csv <<EOF
.headers on
.mode csv
SELECT * FROM accounts ORDER BY cummulative_atoms DESC LIMIT 1000;
EOF


echo "Examples of summaries with various filters."
sqlite3 db.sqlite <<EOF
.headers on
.mode column
.width 30
SELECT

       "nofilter" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(cummulative_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT cummulative_atoms FROM accounts
       ) UNION ALL SELECT

       "only-stakers" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(cummulative_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT cummulative_atoms FROM accounts WHERE staked_atoms > 0
       ) UNION ALL SELECT

       "only-staking" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(staked_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT staked_atoms FROM accounts WHERE staked_atoms > 0
       ) UNION ALL SELECT

       "atom>=20,whalecap-10k" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(capped) AS INTEGER) AS TOTAL FROM (
            SELECT
     CASE WHEN cummulative_atoms > 10000 THEN 10000 ELSE cummulative_atoms END as capped
      FROM accounts
      WHERE capped >= 20

       ) UNION ALL SELECT
       "atom>=20,whalecap-10k,filtered" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(capped) AS INTEGER) AS TOTAL FROM (
            SELECT
     CASE WHEN cummulative_atoms > 10000 THEN 10000 ELSE cummulative_atoms END as capped
      FROM accounts
      WHERE capped >= 20
      AND tag = ""

       ) UNION ALL SELECT
       "atom>=50,whalecap-10k" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(capped) AS INTEGER) AS TOTAL FROM (
            SELECT
     CASE WHEN cummulative_atoms > 10000 THEN 10000 ELSE cummulative_atoms END as capped
      FROM accounts
      WHERE capped >= 50

       ) UNION ALL SELECT
       "atom>=10,unfiltered" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(cummulative_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT cummulative_atoms
      FROM accounts
      WHERE cummulative_atoms >= 10

       ) UNION ALL SELECT
       "atom>=10,filtered" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(cummulative_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT cummulative_atoms
      FROM accounts
      WHERE cummulative_atoms >= 10
      AND tag = ""

       ) UNION ALL SELECT
       "atom>=20,unfiltered" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(cummulative_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT cummulative_atoms
      FROM accounts
      WHERE cummulative_atoms >= 20

       ) UNION ALL SELECT
       "atom>=20,filtered" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(cummulative_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT cummulative_atoms
      FROM accounts
      WHERE cummulative_atoms >= 20
      AND tag = ""

       ) UNION ALL SELECT
       "atom>=50,unfiltered" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(cummulative_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT cummulative_atoms
      FROM accounts
      WHERE cummulative_atoms >= 50

       ) UNION ALL SELECT
       "atom>=50,filtered" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(cummulative_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT cummulative_atoms
      FROM accounts
      WHERE cummulative_atoms >= 50
      AND tag = ""

       ) UNION ALL SELECT
       "atom>=100,unfiltered" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(cummulative_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT cummulative_atoms
      FROM accounts
      WHERE cummulative_atoms >= 100

       ) UNION ALL SELECT
       "atom>=100,filtered" as TITLE,
       COUNT(*) as "PEOPLE", CAST(SUM(cummulative_atoms) AS INTEGER) AS TOTAL FROM (
            SELECT cummulative_atoms
      FROM accounts
      WHERE cummulative_atoms >= 100
      AND tag = ""

       );
EOF

echo "Done."
