-- Can the target user create the extension? One line:
--   available | trusted | installed
-- available: PostgreSQL Flex offers it (count of versions)
-- trusted:   a user without superuser may create it (t/f)
-- installed: it exists in this database already (count)
--
-- Variable: ext (psql -v ext=pg_trgm)

SELECT
    (SELECT count(*) FROM pg_available_extensions WHERE name = :'ext'),
    (SELECT coalesce(bool_or(trusted), false) FROM pg_available_extension_versions WHERE name = :'ext'),
    (SELECT count(*) FROM pg_extension WHERE extname = :'ext');
