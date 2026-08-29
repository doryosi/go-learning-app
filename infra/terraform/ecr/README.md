# Amazon ECR repositories

This Terraform root module creates the DOR-27 container registry foundation:

- one private repository for the Go API;
- one private repository for the worker;
- immutable image tags;
- basic image scanning on every push;
- AES-256 encryption managed by Amazon ECR;
- lifecycle policies for untagged and older images; and
- repository names, ARNs, URLs, and registry ID as outputs.

The module manages registry infrastructure only. Docker builds, tags, login,
and pushes belong to the DOR-26 Taskfile workflow and are intentionally absent
from Terraform.

## Why this is a separate root module

ECR is a regional service and does not live inside the VPC. Keeping this module
separate from `infra/terraform/network` lets registry and network resources be
planned, applied, and destroyed independently. Each root module therefore has
its own state and provider lock file.

## Naming and version policy

The default repository names are:

```text
go-saas-learning-lab-lab-api
go-saas-learning-lab-lab-worker
```

Tags are immutable: after DOR-26 pushes a version such as `v0.2.0`, ECR rejects
another image pushed under the same tag. Publish a new version instead of
silently changing what an existing deployment version means.

## Lifecycle and deletion safety

Each repository expires untagged images after one day and retains the ten most
recent images by default. ECR evaluates lifecycle policies asynchronously, so
an eligible image is not necessarily removed immediately.

`force_delete` defaults to `false`. Terraform will refuse to destroy a
non-empty repository, protecting published images from accidental deletion.
For deliberate lab teardown, first remove the images or temporarily set
`force_delete = true`, inspect the plan, and then apply or destroy.

## Workflow

Create a local variables file if customization is needed:

```sh
cp infra/terraform/ecr/terraform.tfvars.example \
  infra/terraform/ecr/terraform.tfvars
```

Initialize, format, validate, and plan from the repository root:

```sh
task terraform:ecr:init
task terraform:ecr:fmt
task terraform:ecr:validate
task terraform:ecr:plan
```

Read the plan carefully, then apply deliberately:

```sh
terraform -chdir=infra/terraform/ecr apply
```

Inspect the resulting repository URLs:

```sh
terraform -chdir=infra/terraform/ecr output repository_urls
aws ecr describe-repositories --region eu-west-1
```

After images are pushed in DOR-26, inspect tags, digests, and scan status with:

```sh
aws ecr describe-images \
  --region eu-west-1 \
  --repository-name go-saas-learning-lab-lab-api
```

To remove an empty lab registry module:

```sh
terraform -chdir=infra/terraform/ecr destroy
```

Local state and `terraform.tfvars` remain ignored by Git. The provider lock file
is committed so other machines select the same verified provider package.
