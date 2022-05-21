#!/bin/sh

# Input.

GAIAD_BIN=${GAIAD_BIN:-gaiad}
PROP=${PROP:-69}

# Compute context.

LAST_HEIGHT=$(${GAIAD_BIN} status 2>&1 | jq -r .SyncInfo.latest_block_height)
LAST_TIME=$(${GAIAD_BIN} query block $LAST_HEIGHT | jq -r .block.header.time)
LAST_TIMESTAMP=$(date -d $LAST_TIME +%s)

START_TIME=$(${GAIAD_BIN} query gov proposal ${PROP} --output=json | jq -r .voting_start_time)
START_TIMESTAMP=$(date -d $START_TIME +%s)

END_TIME=$(${GAIAD_BIN} query gov proposal ${PROP} --output=json | jq -r .voting_end_time)
END_TIMESTAMP=$(date -d $END_TIME +%s)

find_height_by_date() {
    TARGET=$1
    DIFF_TIMESTAMP=$(expr $LAST_TIMESTAMP - $TARGET)
    DIFF_HEIGHT_ESTIMATION=$(expr $DIFF_TIMESTAMP / 7)
    TARGET_BLOCK_ESTIMATION=$(expr $LAST_HEIGHT - $DIFF_HEIGHT_ESTIMATION)
    echo $TARGET_BLOCK_ESTIMATION
}

START_HEIGHT=$(expr $(find_height_by_date ${START_TIMESTAMP}) - 5000) # FIXME: more accurate
END_HEIGHT=$(expr $(find_height_by_date ${END_TIMESTAMP}) + 1000)     # FIXE: more accurate
START_HEIGHT_ESTIMATE_TIME=$(${GAIAD_BIN} query block $START_HEIGHT | jq -r .block.header.time)
END_HEIGHT_ESTIMATE_TIME=$(${GAIAD_BIN} query block $END_HEIGHT | jq -r .block.header.time)
EVENTS_NB=$(expr $END_HEIGHT - $START_HEIGHT)

echo "[+] prop start time:         $START_TIME   ($START_TIMESTAMP)"
echo "[+] start height (estimate): $START_HEIGHT   ($START_HEIGHT_ESTIMATE_TIME)"
echo "[+] prop end time:           $END_TIME   ($END_TIMESTAMP)"
echo "[+] end height (estimate):   $END_HEIGHT   ($END_HEIGHT_ESTIMATE_TIME)"
echo "[+] events to process:       $EVENTS_NB"

# Search events.

for bblock in $(seq $START_HEIGHT $END_HEIGHT); do
    $GAIAD_BIN query txs --height=$block --limit=100 --events="message.action=vote" --output=json | jq .
done
