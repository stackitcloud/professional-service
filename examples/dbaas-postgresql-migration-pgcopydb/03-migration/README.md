# Stage 3: Migration

Opens the enterprise firewall towards PostgreSQL Flex and puts the pgcopydb runner on the
source VM. It does not start the migration, see the runbook in the
[top-level README](../README.md#migration-runbook).

<table>
  <thead>
    <tr>
      <th>File / directory</th>
      <th>Contents</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>025-remote-state.tf</code></td>
      <td>Outputs of <code>01-source</code> and <code>02-postgres-flex</code></td>
    </tr>
    <tr>
      <td><code>030-firewall.tf</code></td>
      <td>DNS lookup of the instance, egress rule TCP 5432 per address</td>
    </tr>
    <tr>
      <td><code>040-runner.tf</code></td>
      <td>Uploads <code>runner/</code> and <code>migration.env</code> over SSH, builds the image</td>
    </tr>
    <tr>
      <td><code>templates/migration.env.tftpl</code></td>
      <td>Connection data and pgcopydb settings, rendered with both passwords</td>
    </tr>
    <tr>
      <td><code>runner/Containerfile</code></td>
      <td><code>postgres:&lt;target major&gt;-bookworm</code> plus pgcopydb and the CA bundle</td>
    </tr>
    <tr>
      <td><code>runner/scripts/</code></td>
      <td><code>lib.sh</code> and the numbered steps, see <a href="runner/scripts/README.md">runner/scripts/README.md</a></td>
    </tr>
    <tr>
      <td><code>runner/sql/</code></td>
      <td>The SQL the steps run, one query per file</td>
    </tr>
  </tbody>
</table>

## The firewall rule "opening"

PostgreSQL Flex hands out a DNS name, a security group rule needs addresses. The `dns`
provider resolves the name at plan time and creates one `/32` rule per address.


## pgcopydb settings which will be written into `migration.env`

<table>
  <thead>
    <tr>
      <th>Variable</th>
      <th>Default</th>
      <th>Meaning</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>table_jobs</code></td>
      <td><code>4</code></td>
      <td>Tables copied in parallel</td>
    </tr>
    <tr>
      <td><code>index_jobs</code></td>
      <td><code>2</code></td>
      <td>Indexes built in parallel on the target, at most its vCPUs</td>
    </tr>
    <tr>
      <td><code>split_tables_larger_than</code></td>
      <td><code>200MB</code></td>
      <td>Tables above this size are copied in parallel parts</td>
    </tr>
  </tbody>
</table>
