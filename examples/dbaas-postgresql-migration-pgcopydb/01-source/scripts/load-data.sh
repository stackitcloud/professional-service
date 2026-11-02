#!/usr/bin/env bash
# Fills appdb with dummy data until the database reaches the target size.
# Usage (on the source VM):  sudo /opt/pg-source/load-data.sh [TARGET_GB]   (default 1)
set -Eeuo pipefail

TARGET_GB="${1:-1}"
TARGET_BYTES=$(awk -v g="$TARGET_GB" 'BEGIN{printf "%.0f", g*1024*1024*1024}')

EVENTS_PER_BATCH=300000
REVIEWS_PER_BATCH=200000

pex() { podman exec -i pg-source psql -v ON_ERROR_STOP=1 -U postgres -d appdb "$@"; }
dbsize() { pex -tAc "SELECT pg_database_size('appdb')"; }

podman exec pg-source pg_isready -U postgres -d appdb >/dev/null 2>&1 || {
    echo "Container pg-source is not running or not ready: systemctl status pg-source" >&2
    exit 1
}

echo "Target: ${TARGET_GB} GB (${TARGET_BYTES} bytes)"
echo "Start:  $(pex -tAc "SELECT pg_size_pretty(pg_database_size('appdb'))")"

batch=0
while true; do
    size=$(dbsize)
    if (( size >= TARGET_BYTES )); then break; fi

    batch=$((batch + 1))
    printf 'Batch %02d ... ' "$batch"

    pex -q <<-SQL
		-- Spread events over 2025 so every partition gets data.
		INSERT INTO sales.events (customer_id, event_type, payload, created_at)
		SELECT
		    1 + (random() * 49999)::bigint,
		    (ARRAY['page_view','add_to_cart','checkout','search','login'])[1 + (g % 5)],
		    jsonb_build_object(
		        'session', md5(random()::text),
		        'ip',      '10.' || (g % 255) || '.' || ((g * 7) % 255) || '.' || ((g * 13) % 255),
		        'agent',   (ARRAY['chrome','firefox','safari','curl'])[1 + (g % 4)],
		        'ms',      (random() * 5000)::int,
		        'ctx',     md5(g::text) || md5((g + 1)::text)
		    ),
		    '2025-01-01'::timestamptz + (random() * 364.9) * interval '1 day'
		FROM generate_series(1, ${EVENTS_PER_BATCH}) g;

		-- ~256 bytes of text stay inline (no TOAST, no compression), so the
		-- resulting size is predictable.
		INSERT INTO catalog.product_reviews (product_id, rating, title, body, created_at)
		SELECT
		    1 + (random() * 19999)::bigint,
		    1 + (random() * 4)::int,
		    'Review ' || md5(random()::text),
		    md5(random()::text) || md5(random()::text) || md5(random()::text) || md5(random()::text)
		      || md5(random()::text) || md5(random()::text) || md5(random()::text) || md5(random()::text),
		    '2025-01-01'::timestamptz + (random() * 364.9) * interval '1 day'
		FROM generate_series(1, ${REVIEWS_PER_BATCH}) g;
	SQL

    pex -tAc "SELECT pg_size_pretty(pg_database_size('appdb'))"
done

echo "VACUUM ANALYZE ..."
pex -q -c "VACUUM (ANALYZE) sales.events, catalog.product_reviews;"

"$(dirname "$0")/size-report.sh"
