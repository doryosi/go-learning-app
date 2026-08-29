# Milestone 2: AWS Networking Walkthrough

This guide explains the DOR-7 Terraform network foundation. The configuration
does not create resources until someone deliberately runs `terraform apply`.

## Architecture

```text
Internet
   │
Internet Gateway
   │
Public route table (0.0.0.0/0 → IGW)
   │
   ├── Public subnet, AZ A ── future ALB / optional NAT
   └── Public subnet, AZ B ── future ALB / optional NAT
                                │
                         ALB security group
                                │ port 8080 only
                         App security group
                                │
   ├── Private subnet, AZ A ── future EC2
   └── Private subnet, AZ B ── future EC2
```

The public subnets hold internet-facing infrastructure. Application compute
belongs in private subnets and does not receive public IP addresses.

## Terraform root module

The module lives in `infra/terraform/network`:

```text
versions.tf              Terraform and AWS provider versions
variables.tf             User-configurable inputs and validation
main.tf                  VPC, subnet, routing, NAT, and firewall resources
outputs.tf               IDs consumed by later infrastructure
terraform.tfvars.example Safe example customization
README.md                Commands, costs, and packet paths
.terraform.lock.hcl      Exact provider selection and checksums
```

Terraform reads all `.tf` files in a directory as one module. File names help
humans organize the configuration; they do not change Terraform's evaluation
order. Dependencies come from references such as `aws_vpc.this.id`.

## Provider and tags

`versions.tf` constrains Terraform to the supported major version and pins the
AWS provider to the 6.x release family. The generated lock file records the
exact selected provider version and package checksums so other machines install
the same dependency by default.

The AWS provider receives its region from a variable. Credentials are not
stored in code. It also applies default tags to every taggable resource:

- `Project` groups the learning lab.
- `Environment` distinguishes lab or later environments.
- `ManagedBy=Terraform` warns against manual editing.
- `CostCenter=learning-lab` supports cost filtering.

## VPC and CIDR range

The VPC uses `10.0.0.0/16` by default. This private address range contains
65,536 addresses before AWS subnet reservations. DNS support and DNS hostnames
are enabled so later AWS resources can resolve service and instance names.

The VPC is the network boundary, but it does not make a resource public or
private by itself. Subnet routes and public addresses determine reachability.

## Availability Zones and subnets

The module asks AWS for available zones and selects two by default. Each zone
receives one public and one private `/24` subnet:

```text
AZ A: 10.0.0.0/24 public    10.0.10.0/24 private
AZ B: 10.0.1.0/24 public    10.0.11.0/24 private
```

Using multiple zones lets the later ALB remain reachable when one zone has a
failure. Private compute can also be distributed across zones.

Public subnets enable automatic public IPv4 assignment. Private subnets disable
it. The future ALB spans public subnets, while EC2 instances belong in private
subnets.

## Internet Gateway and route tables

An Internet Gateway attaches to the VPC. The shared public route table contains:

```text
Destination       Target
10.0.0.0/16       local (created automatically by AWS)
0.0.0.0/0         Internet Gateway
```

Associating that table with a subnet makes the subnet public. `0.0.0.0/0`
means every IPv4 destination not matched by a more specific route.

Each private subnet receives its own route table. Without NAT, it contains only
the automatic local VPC route, so resources can communicate within the VPC but
cannot reach the general internet.

## NAT modes

A NAT Gateway lets a private resource initiate outbound internet traffic while
preventing unsolicited inbound internet connections. The module makes this an
explicit cost decision:

| Mode | Behavior |
|---|---|
| `none` | No NAT Gateway and no general private internet egress |
| `single` | One NAT in the first public subnet; cheaper but one failure domain |
| `per_az` | One NAT per zone; resilient routing with a higher hourly cost |

AWS charges per NAT Gateway hour and per GB processed. `none` is the default.
For a short lab needing outbound downloads, `single` can be enabled temporarily
and destroyed afterward.

With NAT enabled, the outbound path is:

```text
Private EC2
  → private subnet route table
  → NAT Gateway in a public subnet
  → Internet Gateway
  → internet destination
```

For `per_az`, each private subnet uses the NAT in the same zone. This avoids a
cross-zone dependency and possible cross-zone data charges.

## Security groups

Security groups are stateful virtual firewalls. Response traffic for an allowed
connection is automatically permitted; a separate reverse-direction rule is
not required.

The ALB security group accepts public TCP traffic on ports 80 and 443. Its
outbound rule permits only application traffic on port 8080 to resources using
the application security group.

The application security group accepts port 8080 only when the source has the
ALB security group. It does not allow public inbound traffic or SSH. This uses
identity-based security-group references instead of trusting a broad subnet
CIDR.

The application group currently permits outbound traffic. Routing still
controls whether that traffic can leave: in a private subnet with NAT mode
`none`, an all-egress security-group rule does not create an internet path.

## Terraform expressions

The module uses `for_each` to create one resource for each Availability Zone.
Resources are keyed by zone name, producing stable addresses such as:

```text
aws_subnet.private["eu-west-1a"]
aws_route_table.private["eu-west-1a"]
```

This is safer to reason about than relying only on numeric positions. Local
values build the subnet maps once and reuse them throughout the module.

Conditional `for_each` maps control NAT resources. With mode `none`, Terraform
receives an empty map and therefore creates zero Elastic IPs, NAT Gateways, and
private default routes.

## Outputs

Outputs expose the VPC, subnet, route-table, NAT, and security-group IDs. DOR-8
can consume these values when it creates the ALB and private EC2 application
without duplicating or rediscovering network decisions.

## Safe workflow

Initialize and validate without creating AWS resources:

```sh
task terraform:init
task terraform:fmt
task terraform:validate
```

After configuring an authenticated AWS identity, inspect it and create a plan:

```sh
aws sts get-caller-identity
task terraform:plan
```

Read every planned resource and cost implication before applying. State and
local `.tfvars` files are ignored because state can contain sensitive data and
local inputs may contain environment-specific information. The provider lock
file is committed because it is a reproducibility artifact, not secret state.
