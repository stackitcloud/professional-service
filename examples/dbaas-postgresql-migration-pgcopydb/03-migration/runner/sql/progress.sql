-- What pgcopydb does on the target right now. 45-progress runs it in a loop
-- while 40-clone is busy.

-- Size of the target database so far.
SELECT pg_size_pretty(pg_database_size(current_database())) AS target_size;

-- Active pgcopydb connections, and whether each one is encrypted.
SELECT a.application_name AS worker,
       s.ssl,
       s.version,
       left(regexp_replace(a.query, '\s+', ' ', 'g'), 60) AS query
FROM pg_stat_activity a
JOIN pg_stat_ssl s USING (pid)
WHERE a.datname = current_database()
  AND a.pid <> pg_backend_pid()
  AND a.backend_type = 'client backend'
  AND a.state <> 'idle'
ORDER BY 1;

-- Tables being copied.
SELECT relid::regclass AS copying,
       tuples_processed AS rows,
       pg_size_pretty(bytes_processed) AS bytes
FROM pg_stat_progress_copy
ORDER BY 1;

-- Indexes being built.
SELECT relid::regclass AS indexing,
       phase
FROM pg_stat_progress_create_index
ORDER BY 1;
