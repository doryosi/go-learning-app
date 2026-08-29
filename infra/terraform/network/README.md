# AWS network foundation

This Terraform root module creates the DOR-7 network foundation:

- One DNS-enabled VPC
- Public and private subnets across two Availability Zones by default
- An Internet Gateway and public default route
- A separate private route table per Availability Zone
- An optional cost-aware NAT Gateway topology
- Security groups for a public ALB and private application compute
- Default project, environment, management, and cost-allocation tags

It does not create EC2 instances or an ALB. Those belong to DOR-8.

## Packet paths

Inbound application traffic will follow this path after DOR-8:

```text
Internet
  → Internet Gateway
  → public subnet
  → Application Load Balancer (ports 80/443)
  → app security group (port 8080 only from the ALB security group)
  → EC2 instance in a private subnet
```

Private outbound traffic depends on `nat_gateway_mode`:

```text
Private EC2
  → private route table
  → NAT Gateway in a public subnet
  → Internet Gateway
  → Internet
```

An Internet Gateway alone does not give a private subnet internet access. A
subnet is public when its route table has a default route to an Internet
Gateway; a private subnet has no such direct route.

## NAT strategy and cost

AWS charges for every hour a NAT Gateway exists and for each GB it processes.
The default is therefore deliberately:

```hcl
nat_gateway_mode = "none"
```

The available modes are:

| Mode | Gateways | Cost and availability tradeoff |
|---|---:|---|
| `none` | 0 | No NAT charge; private resources have no general internet egress |
| `single` | 1 | Lower lab cost; cross-AZ traffic and one egress failure domain |
| `per_az` | One per AZ | Better resilience and avoids cross-AZ NAT routing; highest hourly cost |

For a short experiment that needs package or image downloads, change the mode
to `single`, apply, learn, and destroy promptly. DOR-8 should eventually reduce
the need for direct instance internet access by using managed delivery and
AWS service endpoints where appropriate.

## Authentication

Do not put AWS access keys in Terraform files or commit them to Git. The AWS
provider supports environment variables, shared AWS configuration, SSO, and
role-based credentials. Verify the active identity before planning:

```sh
aws sts get-caller-identity
```

## Workflow

Copy the example values only when customization is needed:

```sh
cp terraform.tfvars.example terraform.tfvars
```

Then initialize and validate:

```sh
task terraform:init
task terraform:fmt
task terraform:validate
```

Review a plan before creating anything:

```sh
task terraform:plan
```

Running `terraform apply` creates billable AWS resources when NAT is enabled.
It is intentionally not wrapped in a Taskfile task so applying remains a
deliberate action. When an experiment is finished, run `terraform destroy` and
confirm in AWS that the resources are gone.

Local state is suitable only for this learning stage. A later collaboration or
CI milestone should move state to a protected remote backend with locking.
