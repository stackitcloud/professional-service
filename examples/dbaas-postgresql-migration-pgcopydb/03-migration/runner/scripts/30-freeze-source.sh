#!/usr/bin/env bash
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
