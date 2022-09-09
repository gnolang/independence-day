#!/bin/sh

rm -f db.sqlite

sqlite3 db.sqlite <<EOF
SELECT "----- CREATE TABLES";
CREATE TABLE accounts (
       address STRING PRIMARY KEY,
       liquid_atoms DOUBLE DEFAULT 0,
       staked_atoms DOUBLE DEFAULT 0,
       unbounding_atoms DOUBLE DEFAULT 0,
       skip_reason string STRING DEFAULT "",
       cummulative_atoms DOUBLE
);
CREATE TABLE uatom_holders (address STRING PRIMARY KEY, quantity DOUBLE);
CREATE TABLE delegations_grouped (address STRING PRIMARY KEY, quantity DOUBLE, count INTEGER);
CREATE TABLE undelegations_grouped (address STRING PRIMARY KEY, quantity DOUBLE, count INTEGER);
CREATE TABLE skip_reasons (address STRING PRIMARY KEY, reason STRING);
.tables

SELECT "----- IMPORT CSV FILES";
.separator ,
.import summaries/uatom_holders.csv uatom_holders
.import summaries/delegations_grouped.csv delegations_grouped
.import summaries/undelegations_grouped.csv undelegations_grouped
.import skip.csv skip_reasons

SELECT "----- CHECK CONTENT OF IMPORTED CSV TABLES";
.mode column
SELECT COUNT(*) FROM uatom_holders;
SELECT * FROM uatom_holders LIMIT 1;
SELECT COUNT(*) FROM delegations_grouped;
SELECT * FROM delegations_grouped LIMIT 1;
SELECT COUNT(*) FROM undelegations_grouped;
SELECT * FROM undelegations_grouped LIMIT 1;
SELECT COUNT(*) FROM undelegations_grouped;
SELECT * FROM undelegations_grouped LIMIT 1;
SELECT COUNT(*) FROM skip_reasons;
SELECT * FROM skip_reasons LIMIT 1;

SELECT "----- AGGREGATE TEMP TABLES";
INSERT INTO accounts(address, liquid_atoms) \
       SELECT address, quantity FROM uatom_holders;
INSERT INTO accounts(address, staked_atoms) \
       SELECT address, quantity FROM delegations_grouped WHERE true \
       ON CONFLICT(address) DO \
       UPDATE SET staked_atoms = excluded.staked_atoms;
INSERT INTO accounts(address, unbounding_atoms) \
       SELECT address, quantity FROM undelegations_grouped WHERE true \
       ON CONFLICT(address) DO \
       UPDATE SET staked_atoms = excluded.unbounding_atoms;
INSERT INTO accounts(address, skip_reason) \
       SELECT address, reason FROM skip_reasons WHERE true \
       ON CONFLICT(address) DO \
       UPDATE SET skip_reason = excluded.skip_reason;
UPDATE accounts SET liquid_atoms = liquid_atoms / 1000000, staked_atoms = staked_atoms / 1000000, unbounding_atoms = unbounding_atoms / 1000000;
UPDATE accounts SET cummulative_atoms = (liquid_atoms + staked_atoms + unbounding_atoms);

SELECT "----- CHECK AGGREGATED TABLE";
.headers on
SELECT COUNT(*), SUM(liquid_atoms), SUM(staked_atoms), SUM(unbounding_atoms), SUM(cummulative_atoms) FROM accounts;
 SELECT * FROM accounts ORDER BY cummulative_atoms DESC LIMIT 10;
.headers off

SELECT "----- CLEANUP";
DROP TABLE uatom_holders;
DROP TABLE delegations_grouped;
DROP TABLE undelegations_grouped;
.tables
EOF
