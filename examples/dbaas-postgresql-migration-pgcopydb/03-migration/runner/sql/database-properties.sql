-- Encoding and collation of the current database, as one line:
--   encoding | collate | ctype | locale provider | collation version
-- 10-preflight compares source and target. datlocprovider and datcollversion
-- exist since PostgreSQL 15.

SELECT pg_encoding_to_char(encoding),
       datcollate,
       datctype,
       datlocprovider,
       coalesce(datcollversion, '-')
FROM pg_database
WHERE datname = current_database();
