-- Roles the source database refers to: owners of tables and schemas, and
-- grantees of tables, schemas and default privileges. One role per line.
-- Used by 10-preflight to find roles without a user on the target.
--
-- PUBLIC (oid 0) drops out at the join with pg_roles. Predefined roles (pg_*)
-- are left out: they exist in every cluster, e.g. pg_database_owner, which
-- owns schema public since PostgreSQL 15.

WITH user_namespaces AS (
    SELECT oid, nspname, nspowner, nspacl
    FROM pg_namespace
    WHERE nspname !~ '^pg_'
      AND nspname NOT IN ('information_schema', 'metric_helpers', 'user_management')
),
user_relations AS (
    SELECT c.relowner, c.relacl
    FROM pg_class c
    JOIN user_namespaces n ON n.oid = c.relnamespace
    WHERE NOT EXISTS (SELECT 1 FROM pg_depend d
                      WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid AND d.deptype = 'e')
),
referenced AS (
          SELECT relowner AS oid                 FROM user_relations
    UNION SELECT (aclexplode(relacl)).grantee    FROM user_relations  WHERE relacl IS NOT NULL
    UNION SELECT nspowner                        FROM user_namespaces WHERE nspname <> 'public'
    UNION SELECT (aclexplode(nspacl)).grantee    FROM user_namespaces WHERE nspacl IS NOT NULL
    UNION SELECT (aclexplode(defaclacl)).grantee FROM pg_default_acl
)
SELECT DISTINCT r.rolname
FROM referenced
JOIN pg_roles r ON r.oid = referenced.oid
WHERE r.rolname !~ '^pg_'
ORDER BY 1;
