-- Extensions of the database with their schema, as name|schema. plpgsql is
-- in every database and left out. Used by 10-preflight and 20-prepare-target.

SELECT e.extname, n.nspname
FROM pg_extension e
JOIN pg_namespace n ON n.oid = e.extnamespace
WHERE e.extname <> 'plpgsql'
ORDER BY 1;
