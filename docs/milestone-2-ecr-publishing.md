# Milestone 2: publish versioned images to Amazon ECR

DOR-26 creates the manual release workflow that publishes the API and worker
images to the repositories provisioned by DOR-27. Terraform owns the registry
infrastructure. Task, Docker, and AWS CLI own application artifact delivery.

## Why Terraform does not push images

Terraform reconciles long-lived infrastructure with declared state. A Docker
image is a release artifact that changes whenever application source changes.
Running Docker commands from Terraform would couple infrastructure plans to
local build tools, make pushes difficult to retry safely, and put release side
effects outside Terraform's resource model.

The manual Task workflow maps directly to the later DOR-13 CI pipeline:

```text
AWS authentication
  → ECR login
  → build
  → tag
  → push
  → verify
```

## Prerequisites

Install and verify the required tools:

```sh
brew install awscli jq
aws --version
docker buildx version
task --version
terraform version
```

Configure AWS credentials outside the repository. This lab uses a named shared
credentials profile:

```sh
aws configure --profile go-learning-lab
aws sts get-caller-identity --profile go-learning-lab
```

The credentials are stored under `~/.aws`, not in the Taskfile, Dockerfile, or
container image. A different profile can be selected per invocation:

```sh
AWS_PROFILE=another-profile IMAGE_TAG="$(git rev-parse --short HEAD)" task ecr:publish
```

## Image tags, Git tags, and platform

ECR tags are immutable. The workflow supports two deliberate naming modes:

- A development image uses the current short commit, such as `fb8684c`.
- An official release uses a semantic version, such as `v0.2.0`, only when the
  matching Git tag already exists and points to the current commit.

This produces an auditable chain for a release:

```text
Git tag v0.2.0 → Git commit → OCI revision label
Docker tag v0.2.0 → immutable ECR image digest
```

The workflow rejects `dev`, a different commit hash, a semantic version without
a matching Git tag, and a Git tag pointing to another commit. It also refuses
to publish when the Dockerfile or Go application inputs have uncommitted
changes, so the OCI revision identifies the exact source used by the build.

The default target is `linux/amd64`, suitable for an x86 EC2 instance even when
building on an Apple Silicon laptop. Override it only when the deployment target
is intentionally ARM64:

```sh
IMAGE_PLATFORM=linux/arm64 IMAGE_TAG="$(git rev-parse --short HEAD)" task ecr:publish
```

## Individual steps

Each stage can be run and debugged independently:

```sh
export IMAGE_TAG="$(git rev-parse --short HEAD)"
task ecr:login
task ecr:build
task ecr:tag
task ecr:push
task ecr:verify
```

Repository addresses come from `terraform output`; the account ID and URLs are
not duplicated in the Taskfile. The login task pipes a temporary, region-bound
ECR password directly into `docker login --password-stdin`.

## Complete publish

Publish a development build using the current short commit:

```sh
IMAGE_TAG="$(git rev-parse --short HEAD)" task ecr:publish
```

For an official release, commit the source, create and push an annotated Git
tag, and use the same semantic version for both images:

```sh
git tag -a v0.2.1 -m "Release v0.2.1"
git push origin v0.2.1
IMAGE_TAG=v0.2.1 task ecr:publish
```

The full publishing command then performs every stage:

```text
task ecr:login → task ecr:build → task ecr:tag → task ecr:push → task ecr:verify
```

The build embeds these OCI labels in both images:

- image version;
- full Git revision;
- UTC creation time; and
- source repository URL.

Verification compares each local image's embedded revision with the current Git
commit and asks ECR for its remote tag, digest, push time, and scanning status.
Scanning can initially report `IN_PROGRESS`; ECR completes it asynchronously.

## Common failures

### The tag already exists

ECR rejects a second push because repository tags are immutable. Choose a new
version instead of overwriting the existing release.

### The image tag does not match the current Git revision

For a development build, use `IMAGE_TAG="$(git rev-parse --short HEAD)"`. For an
official release, create a semantic Git tag on the current commit before using
that version as `IMAGE_TAG`. A semantic Docker tag without its corresponding
Git tag is rejected.

### No basic authentication credentials

Run `ecr:login` again. ECR authorization tokens are regional and expire after
12 hours. Confirm that `AWS_REGION` matches the repository region.

### The source tree is dirty

Commit changes to `Dockerfile`, `go.mod`, `cmd`, or `internal`, then rerun the
workflow. Documentation-only or Taskfile changes do not alter the application
filesystem sent through the Docker build context.

### The image has the wrong architecture

Inspect it with:

```sh
docker image inspect go-learning-app-api:v0.2.0 \
  | jq -r '.[0].Architecture'
```

Rebuild with the platform matching the EC2 instance architecture.

## Cleanup

Logging out removes Docker's cached ECR authorization token:

```sh
docker logout "$(terraform -chdir=infra/terraform/ecr output -json repository_urls \
  | jq -r '.api | split("/")[0]')"
```

Do not delete published version tags merely to reuse them. The lifecycle policy
will clean older and untagged images according to DOR-27's retention rules.
