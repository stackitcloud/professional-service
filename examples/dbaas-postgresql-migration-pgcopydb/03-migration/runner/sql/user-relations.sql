-- Number of user relations (tables, partitioned tables, materialized views,
-- views, sequences) in the database. 10-preflight expects 0 on the target.
-- Same filter for user objects as in fingerprint.sql.

SELECT count(*)
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname !~ '^pg_'
  AND n.nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
  AND NOT EXISTS (SELECT 1 FROM pg_depend d
                  WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype = 'e')
  AND c.relkind IN ('r', 'p', 'm', 'v', 'S');
