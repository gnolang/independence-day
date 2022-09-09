#!/bin/sh

rm -f db.sqlite

sqlite3 db.sqlite <<EOF
SELECT "----- CREATE TABLES";
CREATE TABLE accounts (
       address STRING PRIMARY KEY,
       liquid_uatoms DOUBLE,
       staked_uatoms DOUBLE,
       unbounding_uatoms DOUBLE,
       is_blacklist BOOL
);
CREATE TABLE uatom_holders (address STRING PRIMARY KEY, quantity DOUBLE);
CREATE TABLE delegations_grouped (address STRING PRIMARY KEY, quantity DOUBLE, count INTEGER);
CREATE TABLE undelegations_grouped (address STRING PRIMARY KEY, quantity DOUBLE, count INTEGER);
.tables

SELECT "----- IMPORT CSV FILES";
.separator ,
.import summaries/uatom_holders.csv uatom_holders
.import summaries/delegations_grouped.csv delegations_grouped
.import summaries/undelegations_grouped.csv undelegations_grouped

SELECT "----- CHECK CONTENT OF IMPORTED CSV TABLES";
SELECT COUNT(*) FROM uatom_holders;
SELECT * FROM uatom_holders LIMIT 1;
SELECT COUNT(*) FROM delegations_grouped;
SELECT * FROM delegations_grouped LIMIT 1;
SELECT COUNT(*) FROM undelegations_grouped;
SELECT * FROM undelegations_grouped LIMIT 1;

SELECT "----- AGGREGATE TEMP TABLES";
INSERT INTO accounts(address, liquid_uatoms) \
       SELECT address, quantity FROM uatom_holders;
INSERT INTO accounts(address, staked_uatoms) \
       SELECT address, quantity FROM delegations_grouped WHERE true \
       ON CONFLICT(address) DO \
       UPDATE SET staked_uatoms = excluded.staked_uatoms;
INSERT INTO accounts(address, unbounding_uatoms) \
       SELECT address, quantity FROM undelegations_grouped WHERE true \
       ON CONFLICT(address) DO \
       UPDATE SET staked_uatoms = excluded.unbounding_uatoms;

SELECT "----- CHECK AGGREGATED TABLE";
SELECT COUNT(*) FROM accounts;
SELECT * FROM accounts LIMIT 10;

SELECT "----- CLEANUP";
DROP TABLE uatom_holders;
DROP TABLE delegations_grouped;
DROP TABLE undelegations_grouped;
.tables
EOF
