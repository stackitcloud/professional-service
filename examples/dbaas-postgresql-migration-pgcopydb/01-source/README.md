# Stage 1: Source

Stands in for the customer's data center: a VM with PostgreSQL in Podman behind a
firewall that only allows what a database server typically may do. Run before
[`../02-postgres-flex`](../02-postgres-flex).

<table>
  <thead>
    <tr>
      <th>File / directory</th>
      <th>Contents</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td><code>030-project.tf</code></td>
      <td>Plain project, no network area</td>
    </tr>
    <tr>
      <td><code>040-network.tf</code></td>
      <td>Network, security group as enterprise firewall, removal of the default egress rules, its rules</td>
    </tr>
    <tr>
      <td><code>050-server.tf</code></td>
      <td>Key pair, NIC, public IP, VM with cloud-init</td>
    </tr>
    <tr>
      <td><code>cloud-init/</code></td>
      <td>Podman, Quadlet unit <code>pg-source.container</code>, init scripts</td>
    </tr>
    <tr>
      <td><code>initdb/</code></td>
      <td>Roles, the databases <code>appdb</code> and <code>analyticsdb</code>, schemas and seed data</td>
    </tr>
    <tr>
      <td><code>scripts/</code></td>
      <td><code>load-data.sh</code> (fill <code>appdb</code> to a target size) and <code>size-report.sh</code>, both under <code>/opt/pg-source</code></td>
    </tr>
  </tbody>
</table>


## Check the source

```bash
ssh ubuntu@$(terraform output -raw public_ip)
cloud-init status --wait
systemctl status pg-source
sudo podman exec -it pg-source psql -U postgres -d appdb -c '\dn'
sudo /opt/pg-source/size-report.sh
```

The first boot pulls the image and loads the seed data.

For migration tests we can fill up the database with some dummy data:

```bash
# fill up database "appdb" to 5 GB
sudo /opt/pg-source/load-data.sh 5
```
