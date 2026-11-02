# Stage 2: PostgreSQL Flex

The target: its own project, a PostgreSQL Flex instance, the user pgcopydb writes with,
and one empty database per source database. Run after [`../01-source`](../01-source)
and before [`../03-migration`](../03-migration).

<table>
  <thead>
    <tr>
      <th>File</th>
      <th>Contents</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>025-source-state.tf</code></td>
      <td>Public IP and database list from <code>01-source</code>, or from variables</td>
    </tr>
    <tr>
      <td><code>030-project.tf</code></td>
      <td>Target project</td>
    </tr>
    <tr>
      <td><code>040-postgresflex.tf</code></td>
      <td>Instance, owner user <code>app_owner</code> (<code>login</code>, <code>createdb</code>), application users <code>app_rw</code> and <code>app_ro</code> (<code>login</code>), databases</td>
    </tr>
    <tr>
      <td><code>050-outputs.tf</code></td>
      <td>Host, port, credentials, sizing for <code>03-migration</code></td>
    </tr>
  </tbody>
</table>

## Users

Two kinds of users, so that the application cannot change the structure of the database:

<table>
  <thead>
    <tr>
      <th>User</th>
      <th>Roles</th>
      <th>Owns</th>
      <th>Used by</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>app_owner</code> (<code>owner_user</code>)</td>
      <td><code>login</code>, <code>createdb</code></td>
      <td>the databases and every object in them</td>
      <td>the migration, later schema changes and deployments</td>
    </tr>
    <tr>
      <td><code>app_rw</code>, <code>app_ro</code> (<code>app_users</code>)</td>
      <td><code>login</code></td>
      <td>nothing</td>
      <td>the application, with the grants of the source role of the same name</td>
    </tr>
  </tbody>
</table>


## Without 01-source

Set both variables and the state of `01-source` is not read:

```hcl
source_egress_cidrs = ["198.51.100.20/32"]   # what your migration host looks like from the internet
databases           = ["appdb", "analyticsdb"]
```
