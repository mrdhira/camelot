#!/usr/bin/env bash
# Serves Camelot on a fresh copy of testdata/vault with fixture cards, for the Playwright suite.
set -euo pipefail
here=$(cd "$(dirname "$0")/.." && pwd)
repo=$(cd "$here/../.." && pwd)
vault="$here/.vault"
rm -rf "$vault" && cp -R "$repo/testdata/vault" "$vault"
export CAMELOT_VAULT="$vault" CAMELOT_LISTEN=127.0.0.1:7711 CAMELOT_LOG_LEVEL=warn CAMELOT_ACTOR=fixture
bin="$repo/bin/camelot"
"$bin" task create --id M9 --type milestone --title "Fixture milestone" --status wip >/dev/null
"$bin" task create --id FIX-010 --title "Card to drag" --status todo --milestone M9 --docs TRD900 >/dev/null
"$bin" task create --id FIX-011 --title "Card waiting on Dhira" --status review --waiting "Approve the fixture?" --options "Approve|Rework the plan" --folder tasks/DITA-900-fixture >/dev/null
"$bin" task create --id FIX-012 --type security-debt --title "Blocked card" --deps FIX-010 >/dev/null
"$bin" task create --id FIX-013 --title "Card to move by keyboard" --status backlog >/dev/null
"$bin" task create --id FIX-015 --title "Card for a written answer" --status review --waiting "Ship it?" >/dev/null
"$bin" task create --id FIX-016 --title "Card for the panel" --status todo >/dev/null
exec "$bin" serve
