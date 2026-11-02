-- Objects of the source that need attention, as one line:
--   unlogged tables | materialized views | large objects
-- Used by 10-preflight. Same filter for user objects as in fingerprint.sql.

WITH user_relations AS (
    SELECT c.relkind, c.relpersistence
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname !~ '^pg_'
      AND n.nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
)
SELECT
    (SELECT count(*) FROM user_relations WHERE relkind IN ('r', 'p') AND relpersistence = 'u'),
    (SELECT count(*) FROM user_relations WHERE relkind = 'm'),
    (SELECT count(*) FROM pg_largeobject_metadata);
