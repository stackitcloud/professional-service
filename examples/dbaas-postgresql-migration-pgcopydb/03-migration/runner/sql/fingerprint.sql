-- Normalized description of one database. 40-clone takes it from the frozen
-- source (source.txt), 60-verify from the target (target.txt). The two must be
-- identical. Every line starts with its section:
--
--   obj     tables, partitions, indexes, sequences, views, with persistence
--   rows    exact row count per table
--   seq     last value per sequence
--   con     constraints
--   trg     user triggers
--   fn      functions
--   acl     privileges of the application users
--   defacl  default privileges of the application users
--
-- Owners are left out, they differ on purpose (--no-owner).
--
-- User objects only, on both sides:
--   n.nspname !~ '^pg_'   system schemas (pg_catalog, pg_toast, pg_temp_*)
--   NOT IN (...)          information_schema, and what STACKIT puts into
--                         every database (metric_helpers, user_management)
--   pg_depend 'e'         objects that belong to an extension
--
-- Variable: app_users, comma separated (psql -v app_users=app_rw,app_ro)

SELECT 'obj', n.nspname, c.relkind, c.relname, c.relpersistence
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname !~ '^pg_'
  AND n.nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
  AND NOT EXISTS (SELECT 1 FROM pg_depend d
                  WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype = 'e')
  AND c.relkind IN ('r', 'p', 'm', 'v', 'S', 'i', 'I')
ORDER BY 2, 3, 4;

-- Partitions are counted once, through their parent table.
SELECT 'rows', n.nspname || '.' || c.relname,
       CASE WHEN c.relkind = 'm' AND NOT c.relispopulated THEN 'unpopulated'
            ELSE (xpath('/row/cnt/text()',
                        query_to_xml(format('SELECT count(*) AS cnt FROM %I.%I', n.nspname, c.relname),
                                     false, true, '')))[1]::text
       END
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname !~ '^pg_'
  AND n.nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
  AND NOT EXISTS (SELECT 1 FROM pg_depend d
                  WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype = 'e')
  AND c.relkind IN ('r', 'p', 'm')
  AND NOT c.relispartition
ORDER BY 2;

SELECT 'seq', schemaname || '.' || sequencename, coalesce(last_value::text, 'unused')
FROM pg_sequences
WHERE schemaname NOT IN ('metric_helpers', 'user_management')
ORDER BY 2;

SELECT 'con', n.nspname || '.' || c.conname, c.contype
FROM pg_constraint c
JOIN pg_namespace n ON n.oid = c.connamespace
WHERE n.nspname !~ '^pg_'
  AND n.nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
ORDER BY 2;

SELECT 'trg', n.nspname || '.' || c.relname, t.tgname
FROM pg_trigger t
JOIN pg_class c ON c.oid = t.tgrelid
JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE NOT t.tgisinternal
  AND n.nspname !~ '^pg_'
  AND n.nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
ORDER BY 2, 3;

SELECT 'fn', n.nspname || '.' || p.proname, pg_get_function_identity_arguments(p.oid)
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname !~ '^pg_'
  AND n.nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
  AND NOT EXISTS (SELECT 1 FROM pg_depend d
                  WHERE d.classid = 'pg_proc'::regclass AND d.objid = p.oid AND d.deptype = 'e')
ORDER BY 2, 3;

-- Privileges on tables, sequences and views, then on schemas.
SELECT 'acl', r.rolname, n.nspname || '.' || c.relname, a.privilege_type
FROM pg_class c
JOIN pg_namespace n ON n.oid = c.relnamespace
CROSS JOIN LATERAL aclexplode(c.relacl) a
JOIN pg_roles r ON r.oid = a.grantee
WHERE n.nspname !~ '^pg_'
  AND n.nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
  AND NOT EXISTS (SELECT 1 FROM pg_depend d
                  WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype = 'e')
  AND r.rolname = ANY (string_to_array(:'app_users', ','))
UNION ALL
SELECT 'acl', r.rolname, n.nspname, a.privilege_type
FROM pg_namespace n
CROSS JOIN LATERAL aclexplode(n.nspacl) a
JOIN pg_roles r ON r.oid = a.grantee
WHERE n.nspname !~ '^pg_'
  AND n.nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
  AND r.rolname = ANY (string_to_array(:'app_users', ','))
ORDER BY 2, 3, 4;

-- Default privileges without the role they are defined for: postgres on the
-- source, the owner user on the target. defaclobjtype is of type "char" and
-- needs the cast for ||.
SELECT 'defacl', r.rolname, coalesce(n.nspname, '*') || ':' || d.defaclobjtype::text, a.privilege_type
FROM pg_default_acl d
LEFT JOIN pg_namespace n ON n.oid = d.defaclnamespace
CROSS JOIN LATERAL aclexplode(d.defaclacl) a
JOIN pg_roles r ON r.oid = a.grantee
WHERE r.rolname = ANY (string_to_array(:'app_users', ','))
ORDER BY 2, 3, 4;
