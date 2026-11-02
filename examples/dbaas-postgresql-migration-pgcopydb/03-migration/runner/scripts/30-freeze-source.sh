#!/usr/bin/env bash
# Copyright 2026 Schwarz Digits Cloud GmbH & Co. KG
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# write freeze on the source. pgcopydb copies one consistent snapshot,
# so everything written after the start of 40-clone would be lost.
#
# This sets the database to read-only for new sessions and ends the open ones.
# A session can still switch back with SET, so for production the application
# has to be stopped as well. How the freeze is done is the DB team's call.
#
# Usage:  sudo ./30-freeze-source.sh <database> on    # before 40-clone
#         sudo ./30-freeze-source.sh <database> off   # rollback
source "$(dirname "$0")/lib.sh"
need_db "$@"
mode="${2:-}"

# ALTER DATABASE cannot run from inside the database it changes once that is
# read-only, so it goes through the maintenance database.
case "$mode" in
    on)
        src_psql postgres -c "ALTER DATABASE $(ident "$DB") SET default_transaction_read_only = on"
        ended=$(src_psql postgres -c "SELECT count(pg_terminate_backend(pid)) FROM pg_stat_activity
            WHERE datname = $(lit "$DB") AND pid <> pg_backend_pid()")
        echo "Ended $ended open session(s) on $DB."
        ;;
    off)
        src_psql postgres -c "ALTER DATABASE $(ident "$DB") RESET default_transaction_read_only"
        ;;
    *)
        die "usage: $(basename "$0") <database> on|off"
        ;;
esac

echo "default_transaction_read_only on $DB: $(src_psql "$DB" -c 'SHOW default_transaction_read_only')"
[[ "$mode" == on ]] && echo "Next: ./40-clone.sh $DB"
exit 0
