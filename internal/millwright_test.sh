#!/usr/bin/env bash

allComponents=('ingestion' 'dispatcher' 'handler_cpu_usage' 'handler_load' 'handler_kernel_upgrade')

MILLWRIGHT=./internal/mw

function check_health {
  for t in "${allComponents[@]}"; do
    "$MILLWRIGHT" inspect "$t" >/dev/null || exit 1
  done
}

function kill_all {
  for t in "${allComponents[@]}"; do
    "$MILLWRIGHT" kill "$t" >/dev/null || exit 1
  done
}

echo 'Starting test.'

echo 'Building internal binary.'
go build -o mw; cd ..

echo 'Doing a preliminary cleanup.'
"$MILLWRIGHT" destroy

echo 'Starting internal.'
# --force just in case an internal is already running.
# Would have liked to output only stderr but logrus logs INFO to stderr for some reason so silecing both.
"$MILLWRIGHT" start --force &>/dev/null &
export MW_PID=$!

echo 'Waiting up to 5 minutes for the internal to finish initialization.'
sleep 300

echo 'Checking components'\'' health.'

check_health

echo 'Testing recovery of 1 failure while internal is running.'

"$MILLWRIGHT" kill ingestion

sleep 15
check_health

echo 'Testing recovery of multiple failures while internal is running.'

kill_all

sleep 20
check_health

echo 'Testing recovery of multiple failures from new internal.'

kill -INT "$MW_PID"
kill_all

# It should be able to start without --force now.
"$MILLWRIGHT" start &>/dev/null &
export MW_PID=$!

sleep 30
check_health

echo 'Testing recovery of network misconfiguration from new internal.'

kill -INT "$MW_PID"

docker network disconnect -f millwright-bridge dispatcher

"$MILLWRIGHT" start &>/dev/null &
export MW_PID=$!

sleep 15
check_health

echo 'All tests passed.'

echo 'Cleaning up.'

kill -INT "$MW_PID"
"$MILLWRIGHT" destroy

echo 'Exiting.'
