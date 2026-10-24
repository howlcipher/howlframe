#!/usr/bin/env bash
S=$1; L=$S/logs; BIN=$S/bin/howlframe
until grep -q '^DONE' $L/summary.txt; do sleep 10; done
run() { local n=$1; shift; local t0=$(date +%s); "$@" >"$L/$n.log" 2>&1; local rc=$?; echo "$n rc=$rc secs=$(( $(date +%s)-t0 )) cmd=$*" >> "$L/summary2.txt"; }
: > $L/summary2.txt
cd $S/hf
run evidencedesk env HOWLFRAME_BIN=$BIN HOWLBOARD_REPO=$S/hb bash milestone3/evidencedesk/scripts/test.sh
run evidencedesk_gitstatus git -C $S/hf status --porcelain
cd $S/hb
cp $BIN ./howlframe_bin
export HOWL_FORBID_LOCAL_INFERENCE=1
run hb_make_test make test HOWLFRAME_BIN=./howlframe_bin
head -n 500 data/fixtures/ledger_sample.jsonl > $S/probes/slice.jsonl
run hb_make_import make import HOWLFRAME_BIN=./howlframe_bin LEDGER=$S/probes/slice.jsonl MAX=500
run hb_make_import_full make import HOWLFRAME_BIN=./howlframe_bin
run hb_test_docs python3 scripts/test_docs.py
run hb_test_seo python3 scripts/test_seo.py
run hb_timeline python3 scripts/test_timeline_layout.py
run hb_gitstatus git -C $S/hb status --porcelain
cd $S/hf
run sec_fs go test -count=1 -v -run 'Scoped|FileRootGrant|FileStore|AbsoluteInRootSymlink|RaceHarness|OpenScope' ./internal/vm/ ./internal/capability/
run sec_exec go test -count=1 -v -run 'TestExec' ./internal/vm/
run sec_slow go test -count=1 -v -run 'Slow' ./internal/vm/
run sec_env go test -count=1 -v -run 'Env|CodeLoading|Loader' ./internal/capability/ ./internal/backend/gogen/ ./internal/backend/javascript/ .
run sec_hf05 go test -count=1 -v -run 'ValuePosition|VoidInValue|Void' . ./internal/checker/ ./internal/hfir/
run sec_ticketdesk_restart env APP_ARTIFACT=$S/probes/ticketdesk.hfbc bash -c "$BIN build milestone2/ticketdesk/src/ticketdesk.howl -o $S/probes/ticketdesk.hfbc && python3 milestone2/ticketdesk/tests/regression/restart_default_budget.py milestone2/ticketdesk/tests/acceptance/launcher.example.sh"
echo DONE >> $L/summary2.txt
