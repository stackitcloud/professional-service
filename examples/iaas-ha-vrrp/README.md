# Basic High Availability (HA) Setup Using VRRP

This guide provides a comprehensive, step-by-step process for setting up a Basic High Availability (HA)
cluster using the Virtual Router Redundancy Protocol (VRRP) with the STACKIT CLI.
In this HA configuration, one virtual machine (VM) functions as the active primary node while the secondary
remains on standby.

> For setup instructions using the STACKIT CLI instead of Terraform, please refer to the [STACKIT CLI Guide](STACKIT-CLI-GUIDE.md).

## Testing the Setup

After completing the setup, use the [test-setup.sh](test-setup.sh) script to verify that the Apache server is operational
on each machine. Executing this script should yield the following results:

```bash
Performing curl on IP: 193.148.177.243
<center><h1>example01</h1>

Performing curl on IP: 193.148.161.92
<center><h1>example02</h1>

Performing curl on IP: 193.148.169.230
<center><h1>example01</h1>
```

The output indicates a successfully functional VRRP setup.

### Failover Testing

To test failover, stop the master VM and perform another `curl` request to the vIP WAN IP:

```bash
vip01_wan_ip=$(terraform output -raw vip01-wan-ip)
curl $vip01_wan_ip

<center><h1>example02</h1>
```

The response confirms that the fail-over from the master to the backup has occurred.

## Diagrams

- **HA Traffic Flow**:
  ![HA Traffic Flow Diagram](docs/ha.svg)

- **vIP Binding Concept**:
  ![vIP Binding Diagram](docs/vip.svg)
