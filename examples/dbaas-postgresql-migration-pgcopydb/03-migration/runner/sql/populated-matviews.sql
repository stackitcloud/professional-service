-- Materialized views that hold data on the source, as schema|name.
-- 50-post refreshes them on the target if they are empty there.

SELECT n.nspname, c.relname
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind = 'm'
  AND c.relispopulated
  AND n.nspname !~ '^pg_'
  AND n.nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
ORDER BY 1, 2;
