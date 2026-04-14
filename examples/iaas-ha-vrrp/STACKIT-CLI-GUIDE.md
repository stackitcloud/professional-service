## Step 1: Create a STACKIT Network

Create a new network where the VMs and network interfaces will be deployed.

```bash
NETWORKID=$(stackit network create \
  --name demo \
  --ipv4-dns-name-servers "1.1.1.1,8.8.8.8,9.9.9.9" \
  --ipv4-prefix "10.1.2.0/24" \
  -y -o json | jq -r .networkId)
```

---

## Step 2: Configure Security Groups

Create a security group allowing VRRP and ICMP traffic between the two VMs.

Create the security group:

```bash
SECGROUPID=$(stackit security-group create \
  --name VRRP \
  -y -o json | jq -r .id)
```

Add security rules:

```bash
# Allow ICMP (for ping and monitoring)
stackit security-group rule create \
  --security-group-id $SECGROUPID \
  --direction ingress \
  --protocol-name icmp

# Allow VRRP protocol (protocol number 112)
stackit security-group rule create \
  --security-group-id $SECGROUPID \
  --direction ingress \
  --protocol-name vrrp
```

Note: Restrict these rules further in production environments by specifying source CIDRs or specific IPs.

---

## Step 3: Create Network Interfaces

We will create:

- A shared VIP interface (to bind the internal HA IP)
- One interface for each VM with access to the VIP

Create a shared network adapter for the virtual IP:

```bash
VIPNICID=$(stackit network-interface create \
  --network-id $NETWORKID \
  --name vipPort \
  -y -o json | jq -r .id)
```

Fetch the vIP address:

```bash
VIPIP=$(stackit network-interface describe $VIPNICID \
  --network-id $NETWORKID \
  -o json | jq -r .ipv4)
```

Create network interfaces for the VMs (replace <nicName> and <defaultSecGroupId> accordingly):

```bash
NICID=$(stackit network-interface create \
  --network-id $NETWORKID \
  --allowed-addresses $VIPIP \
  --name <nicName> \
  --security-groups $SECGROUPID,<defaultSecGroupId> \
  -y -o json | jq -r .id)
```

Repeat the step above to create a second NIC for the second VM.

---

## Step 4: Create the Virtual Machines

Provision two virtual machines with attached network interfaces and required configuration.

Sample command to create a VM (replace placeholders accordingly):

```bash
stackit server create \
  --boot-volume-performance-class storage_premium_perf4 \
  --boot-volume-size 32 \
  --boot-volume-source-type image \
  --boot-volume-source-id 03e19c6a-d73a-4ba9-96af-4bd03cf905d3 \ # Debian 12 image ID
  --keypair-name <sshKeyPair> \
  --availability-zone eu01-1 \
  --machine-type c1.2 \
  --name <serverName> \
  --network-interface-ids $NICID
```

Repeat the process to create the second VM with a different serverName and NIC ID.

---

## Step 5: Bind a Public IP (Optional — for external access)

To allow access to your HA cluster from outside the private network, bind a public IP address to the shared VIP NIC.

```bash
stackit public-ip create \
  --associated-resource-id $VIPNICID
```

This ensures that regardless of which VM is active, the public IP always routes to the current primary node via the shared virtual IP.
