# Runner scripts

The steps of the migration, run by hand on the migration host (in this example the
source VM). `03-migration` copies them to `/opt/migration/scripts` and builds the runner
image. The order and the commands are in the
[migration runbook](../../../README.md#migration-runbook). This page explains what each
script does and why.

```bash
ssh ubuntu@<public ip of 01-source>
cd /opt/migration/scripts
sudo ./10-preflight.sh appdb
```

<table>
  <thead>
    <tr>
      <th>Script</th>
      <th>Goal</th>
      <th>Writes to</th>
      <th>Safe to repeat</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>00-build-runner.sh</code></td>
      <td>Build the runner image</td>
      <td>host</td>
      <td>yes</td>
    </tr>
    <tr>
      <td><code>10-preflight.sh</code></td>
      <td>Check everything before anything is written</td>
      <td>nothing</td>
      <td>yes</td>
    </tr>
    <tr>
      <td><code>20-prepare-target.sh</code></td>
      <td>Create the extensions on the target</td>
      <td>target</td>
      <td>yes</td>
    </tr>
    <tr>
      <td><code>30-freeze-source.sh</code></td>
      <td>Make the source read-only, or undo that</td>
      <td>source</td>
      <td>yes</td>
    </tr>
    <tr>
      <td><code>40-clone.sh</code></td>
      <td>Copy the database with pgcopydb</td>
      <td>target</td>
      <td>no, reset first</td>
    </tr>
    <tr>
      <td><code>45-progress.sh</code></td>
      <td>Watch a running clone</td>
      <td>nothing</td>
      <td>yes</td>
    </tr>
    <tr>
      <td><code>50-post.sh</code></td>
      <td>Do what pgcopydb leaves out</td>
      <td>target</td>
      <td>yes</td>
    </tr>
    <tr>
      <td><code>60-verify.sh</code></td>
      <td>Prove that source and target are identical</td>
      <td>reports only</td>
      <td>yes</td>
    </tr>
    <tr>
      <td><code>90-reset-target.sh</code></td>
      <td>Empty the target database to start over</td>
      <td>target</td>
      <td>yes</td>
    </tr>
  </tbody>
</table>

Every step except `00` takes the database as first argument and handles one database.
Run the chain once per database.

## How a step runs: lib.sh

`lib.sh` is not run on its own. Every step loads it in its first line, and it does two
things.

**It moves the step into the runner container.** Started on the host, a step does not
run there. `lib.sh` starts the same script again inside a throwaway container from the
runner image, and the host copy ends:

1. Not root? Start again with `sudo`. `migration.env` belongs to root, mode 0600.
2. `podman run --rm` with
   - `--network host`: the source DBMS listens on `127.0.0.1` of the host only
   - `--env-file /opt/migration/migration.env`: hosts, users, passwords, settings
   - `-v /var/lib/pgcopydb:/var/lib/pgcopydb`: logs, reports and the pgcopydb work
     directory, kept between steps. Same path on both sides, so every path a step
     prints can be used on the host as it is
   - `-v /opt/migration/scripts:/scripts:ro`: the scripts themselves
   - `-v /opt/migration/sql:/sql:ro`: the SQL the scripts run, see below
   - `-v /opt/migration/certs:/certs:ro`, if that directory exists: the CA of a source
     with a private CA
   - `-it` when run in a terminal, so prompts and Ctrl-C work
3. `IN_RUNNER=1` tells the second start that it is inside the container now.

So `psql`, `pg_dump` and `pgcopydb` come from the image in the version that matches the
target, and nothing is installed on the host.

**It provides the shared helpers** used by the steps:

<table>
  <thead>
    <tr>
      <th>Helper</th>
      <th>Does</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>need_db</code></td>
      <td>Checks that the argument is one of <code>DATABASES</code>, sets <code>DB</code>, <code>REPORTS</code> and <code>CLONE_DIR</code></td>
    </tr>
    <tr>
      <td><code>src_uri</code>, <code>dst_uri</code></td>
      <td>Connection URIs, user and password URL-encoded. The target always with <code>verify-full</code></td>
    </tr>
    <tr>
      <td><code>src_psql</code>, <code>dst_psql</code></td>
      <td><code>psql</code> against one database with <code>-c</code>, output as <code>a|b|c</code> lines for the scripts to parse</td>
    </tr>
    <tr>
      <td><code>src_sql</code>, <code>dst_sql</code></td>
      <td>The same for a file from <code>sql/</code>, values go in as psql variables (<code>-v ext=pg_trgm</code>)</td>
    </tr>
    <tr>
      <td><code>lit</code>, <code>ident</code></td>
      <td>Quote values and names before they go into an SQL string in a script</td>
    </tr>
    <tr>
      <td><code>APP_USER_LIST</code>, <code>is_app_user</code></td>
      <td>The application users from <code>APP_USERS</code>, as an array and as a test</td>
    </tr>
    <tr>
      <td><code>fingerprint</code></td>
      <td>Runs <code>sql/fingerprint.sql</code>, see <code>40</code> and <code>60</code></td>
    </tr>
    <tr>
      <td><code>referenced_roles</code></td>
      <td>Runs <code>sql/referenced-roles.sql</code>, see <code>10</code></td>
    </tr>
  </tbody>
</table>

## The SQL: sql/

Every query longer than a line is a file of its own in [`sql/`](../sql), with a comment
on what it returns and who uses it. The scripts run them with `src_sql` or `dst_sql`.
Each file can be tried out by hand with `psql`, for example on the source VM:

```bash
sudo podman exec -i pg-source psql -U postgres -d appdb -v app_users=app_rw,app_ro \
  < /opt/migration/sql/fingerprint.sql
```

<table>
  <thead>
    <tr>
      <th>File</th>
      <th>Returns</th>
      <th>Used by</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>fingerprint.sql</code></td>
      <td>Objects, row counts, sequences, constraints, triggers, functions, privileges of the application users</td>
      <td><code>40</code>, <code>60</code></td>
    </tr>
    <tr>
      <td><code>referenced-roles.sql</code></td>
      <td>Roles the database refers to as owner or grantee</td>
      <td><code>10</code></td>
    </tr>
    <tr>
      <td><code>user-relations.sql</code></td>
      <td>Number of user tables, views and sequences</td>
      <td><code>10</code></td>
    </tr>
    <tr>
      <td><code>database-properties.sql</code></td>
      <td>Encoding, collation, locale provider, collation version</td>
      <td><code>10</code></td>
    </tr>
    <tr>
      <td><code>extensions.sql</code></td>
      <td>Extensions with their schema</td>
      <td><code>10</code>, <code>20</code></td>
    </tr>
    <tr>
      <td><code>extension-on-target.sql</code></td>
      <td>Whether an extension is available, trusted, installed</td>
      <td><code>10</code></td>
    </tr>
    <tr>
      <td><code>special-objects.sql</code></td>
      <td>Number of UNLOGGED tables, materialized views, large objects</td>
      <td><code>10</code></td>
    </tr>
    <tr>
      <td><code>populated-matviews.sql</code></td>
      <td>Materialized views that hold data</td>
      <td><code>50</code></td>
    </tr>
    <tr>
      <td><code>progress.sql</code></td>
      <td>Size, active workers with TLS, COPY and index progress</td>
      <td><code>45</code></td>
    </tr>
  </tbody>
</table>

All of them look at user objects only, with the same filter written out in each file:
no system schemas (`pg_*`, `information_schema`), not what STACKIT puts into every
database (`metric_helpers`, `user_management`), and not what belongs to an extension.

## Work directories

The same on the host and in the container. Readable with `sudo`
only:

```
/var/lib/pgcopydb/
├── clone/<db>/      pgcopydb work directory: schema dump, state, its own logs
└── reports/<db>/    source.txt, target.txt, diff.txt, source-grants.sql, clone.log
```

## 00-build-runner.sh

**Goal:** a container image with `psql`, `pg_dump` and `pgcopydb` in the major version
of the target. `03-migration` calls it on every apply that changes the runner, the env
file or the target version. By hand only when you run the runner outside this example.

```bash
sudo PG_MAJOR=18 ./00-build-runner.sh
```

## 10-preflight.sh

**Goal:** find every problem before anything is written. Read-only on both sides, run
it as often as you like. Exits with 1 if a check fails.

```bash
sudo ./10-preflight.sh appdb
```

Every check prints `PASS`, `WARN`, `FAIL` or `INFO`, the summary counts them.

1. **Connectivity.** `SELECT 1` on source and target. If the target fails, the hints
   name the usual causes: the firewall rule of `03-migration` (timeout), the ACL of the
   instance in `02-postgres-flex`, a missing database. Without both connections the
   script stops here.
2. **Versions.** The target must not be older than the source, a downgrade does not
   work with `pg_dump`. `pg_dump` in the runner must be at least the version of the
   source.
3. **Encryption.** Reads `pg_stat_ssl` of its own target connection: TLS on, which
   version. Shows the address the target sees this host as, which is the address
   that has to be in the ACL. For a remote source it checks TLS there as well and warns
   without it.
4. **Target database.** Must not contain user objects yet, otherwise an earlier run is
   still in it (`90-reset-target.sh`). Compares encoding (must match) and collation
   (warning: sort order may change).
5. **Extensions.** For every extension of the source: already on the target, available
   and trusted (then `20` creates it), not available (fail), or not trusted, so the
   owner user cannot create it (fail, ask STACKIT support). Extensions outside
   `public` get a warning, `20` only handles `public`.
6. **Roles and privileges.** Everything on the target will belong to the owner user
   (`TARGET_USER`, `app_owner`). Checks that the owner is not one of the application
   users (`APP_USERS`) and that every application user exists on the target. Warns
   about roles the source grants rights to that have no application user: `50` skips
   their grants.
7. **Objects that need attention.** UNLOGGED tables (content lost on a failover of
   Flex), materialized views (`50` refreshes them), large objects (slow).
8. **Capacity and timing.** Size of the source database against the storage of the
   target: fail above 100 %, warning above 60 %, because WAL and index builds need
   headroom. Shows the backup schedule of the target in UTC next to the current time,
   so the copy does not run into the backup window. Shows whether the source is
   already frozen and how many sessions are open.

## 20-prepare-target.sh

**Goal:** the extensions of the source exist on the target before the copy. pgcopydb
runs with `--skip-extensions`, because without superuser rights it cannot take them over
as they are on the source.

```bash
sudo ./20-prepare-target.sh appdb
```

1. Reads all extensions of the source except `plpgsql` (always there), with their
   schema.
2. Stops if one lives outside `public`: pgcopydb creates every other schema itself and
   fails if it exists already. Such an extension has to be created by hand after the
   schema exists.
3. `CREATE EXTENSION IF NOT EXISTS ... WITH SCHEMA public` on the target for each one.
4. Prints the extensions now on the target, with version.

## 30-freeze-source.sh

**Goal:** nothing is written to the source during the copy. pgcopydb copies one
consistent snapshot. Whatever is written after `40` has started would be missing on the
target.

```bash
sudo ./30-freeze-source.sh appdb on    # before 40-clone
sudo ./30-freeze-source.sh appdb off   # rollback, or after a test run
```

`on`:

1. `ALTER DATABASE appdb SET default_transaction_read_only = on`. Runs through the
   database `postgres`, because a database that is read-only cannot change that setting
   from inside.
2. Ends all open sessions on the database, so that each one reconnects and gets the
   new setting.
3. Prints the setting, as a new session sees it.

`off` resets the setting.

This is a soft freeze: a session can switch back with `SET`. For production stop the
application as well.

## 40-clone.sh

**Goal:** the copy itself. Before it starts, it records what the source looks like (light fingerprint query), so
that `60` has something to compare against.

```bash
sudo ./40-clone.sh appdb
sudo ALLOW_WRITES=1 ./40-clone.sh appdb   # test run without freeze
```

1. Stops if the work directory `clone/<db>` exists: an earlier run, start over with
   `90`.
2. Stops if the source is writable, unless `ALLOW_WRITES=1` is set. Without the freeze
   `60` may report differences, because the source keeps changing.
3. **Fingerprint** of the frozen source to `reports/<db>/source.txt`.
4. **Grants** of the source to `reports/<db>/source-grants.sql`: the `GRANT`, `REVOKE`
   and `ALTER DEFAULT PRIVILEGES` lines from a schema-only `pg_dump`. The clone drops
   them, this file keeps them for `50`.
5. Puts both connection URIs into the environment of pgcopydb. That way the passwords
   do not show up in the process list.
6. **`pgcopydb clone`**, output to the terminal and to `reports/<db>/clone.log`:

   <table>
     <thead>
       <tr>
         <th>Option</th>
         <th>Why</th>
       </tr>
     </thead>
     <tbody>
       <tr>
         <td><code>--no-owner</code></td>
         <td>The owners of the source do not exist on Flex</td>
       </tr>
       <tr>
         <td><code>--no-acl</code></td>
         <td>Grants are applied by <code>50</code>, for the application users only</td>
       </tr>
       <tr>
         <td><code>--skip-extensions</code></td>
         <td><code>20</code> created them</td>
       </tr>
       <tr>
         <td><code>--skip-db-properties</code></td>
         <td><code>ALTER DATABASE ... SET</code> belongs to the instance, not the copy, and would copy the freeze</td>
       </tr>
       <tr>
         <td><code>--table-jobs</code>, <code>--index-jobs</code></td>
         <td>Parallel COPY streams and index builds, from <code>migration.env</code></td>
       </tr>
       <tr>
         <td><code>--split-tables-larger-than</code></td>
         <td>Large tables are copied in several parts at the same time</td>
       </tr>
       <tr>
         <td><code>--fail-fast</code></td>
         <td>Stop at the first error instead of leaving a half copy</td>
       </tr>
     </tbody>
   </table>

   pgcopydb copies the schema without indexes and constraints first, then the data,
   then builds indexes and constraints, resets the sequences, and runs
   `VACUUM ANALYZE`.
7. Prints exit code, duration and the number of `ERROR`/`FATAL` lines in the log. Exits
   with 1 if pgcopydb failed.

## 45-progress.sh

**Goal:** see what a running `40` does, and that every worker connection is encrypted.
Read-only, from a second terminal.

```bash
sudo ./45-progress.sh appdb      # every 5 s
sudo ./45-progress.sh appdb 2    # every 2 s
```

Clears the screen in a loop and shows, from the target:

- the size of the target database
- every active pgcopydb connection with TLS on/off, TLS version and the current query
- `pg_stat_progress_copy`: which table is being copied, rows and bytes so far
- `pg_stat_progress_create_index`: which index is being built, in which phase

Ctrl-C ends it. The clone keeps running.

## 50-post.sh

**Goal:** the two things the clone does not do by itself.

```bash
sudo ./50-post.sh appdb
```

1. **Materialized views.** For every view that holds data on the source: if it is
   empty on the target, `REFRESH MATERIALIZED VIEW`. Already populated ones are left
   alone.
2. **Grants.** The clone ran with `--no-acl`, so the application users have no rights
   yet. For every statement in `source-grants.sql`:
   - grantee is an application user (`APP_USERS`): kept. `ALTER DEFAULT PRIVILEGES FOR
     ROLE <source owner>` becomes `FOR ROLE <owner user>`, because on the target the
     owner user creates new tables.
   - any other grantee, `PUBLIC` included: skipped and listed.

   The kept statements go to `reports/<db>/applied-grants.sql` and are applied in one
   transaction, as the owner user. Repeating the step does no harm, a grant that exists
   already stays as it is.

## 60-verify.sh

**Goal:** proof that the copy is complete, as the condition for the cutover. Two
checks, run the first one always and the second one when the content has to be proven
as well.

```bash
sudo ./60-verify.sh appdb                  # fingerprint, fast
sudo ./60-verify.sh appdb --compare-data   # content of every table, slow
```

### Fingerprint (no option)

1. Stops if `40` did not leave a fingerprint of the source.
2. Takes the same fingerprint from the target to `reports/<db>/target.txt`.
3. `diff` of the two, saved as `reports/<db>/diff.txt`.
4. Identical: prints `IDENTICAL` with the number of objects and rows, plus the
   connection data for the application. Different: prints the diff and exits with 1.

What the fingerprint compares (sorted, so the order of the catalog does not matter):

<table>
  <thead>
    <tr>
      <th>Line</th>
      <th>Content</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>obj</code></td>
      <td>Tables, partitions, indexes, sequences, views, materialized views, with persistence (UNLOGGED)</td>
    </tr>
    <tr>
      <td><code>rows</code></td>
      <td>Exact <code>count(*)</code> per table. Partitioned tables once via the parent. Unpopulated materialized views as <code>unpopulated</code></td>
    </tr>
    <tr>
      <td><code>seq</code></td>
      <td><code>last_value</code> per sequence</td>
    </tr>
    <tr>
      <td><code>con</code></td>
      <td>Constraints with type (primary key, foreign key, unique, check)</td>
    </tr>
    <tr>
      <td><code>trg</code></td>
      <td>User triggers per table</td>
    </tr>
    <tr>
      <td><code>fn</code></td>
      <td>Functions with argument types</td>
    </tr>
    <tr>
      <td><code>acl</code></td>
      <td>Privileges of the application users on tables, sequences, views and schemas</td>
    </tr>
    <tr>
      <td><code>defacl</code></td>
      <td>Default privileges of the application users, without the role they are defined for</td>
    </tr>
  </tbody>
</table>

Owners are not part of it: they differ on purpose (`--no-owner`). Grants of roles that
are not application users are not part of it either.

### Content (`--compare-data`)

The fingerprint counts rows, it does not look at what is in them. This mode runs
`pgcopydb compare data` from the runner image and nothing else:

1. Stops if the source is writable, unless `ALLOW_WRITES=1` is set. The checksums are
   taken from the live source, so it has to be in the state that was copied.
2. Starts with an empty work directory `/var/lib/pgcopydb/compare/<db>`. pgcopydb would
   otherwise reuse the results of an earlier, completed run.
3. `pgcopydb compare data`: for every table on both sides the row count and a checksum
   over the content (the sum of a hash of every row, independent of the order). Output
   to the terminal and to `reports/<db>/compare-data.log`.
4. Exit code 0: prints `DATA IDENTICAL`. Otherwise exits with 1, the log shows which
   table differs.

Every table is read in full on source and target. On a large database that takes a
while, and the write freeze lasts until it is done.
