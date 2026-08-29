# Milestone 1.5: build and run the containers locally

This milestone proves that the API and worker can be built, started, inspected,
and recreated as independent containers before the same images are pushed to
Amazon ECR.

## One Dockerfile, two images

The shared `build` stage compiles the API, worker, and health-check binaries.
The final `api` and `worker` stages then copy only the binaries they need into
separate minimal images.

Build the targets directly with explicit local names and versions:

```sh
docker build --target api -t go-learning-app-api:v0.1.0 .
docker build --target worker -t go-learning-app-worker:v0.1.0 .
```

List and compare the resulting images:

```sh
docker image ls 'go-learning-app-*'
```

Compose performs the same target selection. `IMAGE_TAG` controls the tag for
both images; it defaults to `dev` when omitted:

```sh
export IMAGE_TAG=v0.1.0
export VCS_REVISION="$(git rev-parse HEAD)"
export BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
docker compose build
```

These values become standard OCI image labels. The version is convenient for
humans, while the full Git revision identifies the exact committed source used
for the build. Build from a clean working tree so that the revision describes
all source included in the image. The Taskfile added in DOR-25 calculates the
revision and build time automatically.

Inspect the provenance after building:

```sh
docker image inspect go-learning-app-api:v0.1.0 \
  --format '{{json .Config.Labels}}'

docker image inspect go-learning-app-worker:v0.1.0 \
  --format '{{json .Config.Labels}}'
```

Both images expose `org.opencontainers.image.version`, `revision`, `created`,
and `source`. They use different `title` labels because they are independently
deployable applications.

## Tags, digests, and Git commits

An image tag, Docker digest, and Git commit identify different things:

| Identifier | Example | Meaning |
|---|---|---|
| Image tag | `v0.1.2` | A human-friendly, mutable name for an image |
| Image ID or repository digest | `sha256:b1f11e...` | A content-addressed identity for a built image |
| Git commit | `76d2c7f...` | An immutable snapshot of the source repository |

A tag can be moved to a different image, so it is not sufficient proof of the
image contents. A digest changes when the represented image manifest changes
and is the immutable reference used by container registries and deployment
systems. After the images are pushed to ECR, they can be selected by digest:

```text
123456789012.dkr.ecr.eu-west-1.amazonaws.com/go-api@sha256:...
```

The Docker digest is not the Git commit hash. Docker identifies built image
content, while Git identifies source content. BuildKit provenance and manifest
metadata can also make top-level image digests differ between builds even when
the runtime filesystem is unchanged. We therefore record the Git commit as an
explicit OCI label rather than trying to infer it from the Docker digest.

Show local tags, digests, IDs, creation times, and sizes with:

```sh
docker image ls 'go-learning-app-*' --digests
```

Show the exact image selected by each running container with:

```sh
docker inspect \
  go-learning-app-api-1 \
  go-learning-app-worker-1 \
  --format '{{.Name}} -> {{.Config.Image}} -> {{.Image}}'
```

## Coupling an image to its source commit

The coupling is implemented explicitly in three steps.

First, each final Dockerfile stage declares build arguments with safe fallback
values and stores them as standard OCI labels:

```dockerfile
ARG IMAGE_VERSION=dev
ARG VCS_REVISION=unknown
ARG BUILD_DATE=unknown
ARG SOURCE_URL=https://github.com/doryosi/go-learning-app

LABEL org.opencontainers.image.version="${IMAGE_VERSION}" \
      org.opencontainers.image.revision="${VCS_REVISION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="${SOURCE_URL}"
```

`unknown` is an honest fallback for informal builds that do not supply
provenance. A Dockerfile should not depend on `.git`: the directory is excluded
from the build context, builds may start from source archives, and the build
environment—not the image—knows which revision it is building.

Second, `compose.yml` maps shell variables into Docker build arguments for both
the API and worker targets:

```yaml
args:
  IMAGE_VERSION: ${IMAGE_TAG:-dev}
  VCS_REVISION: ${VCS_REVISION:-unknown}
  BUILD_DATE: ${BUILD_DATE:-unknown}
  SOURCE_URL: ${SOURCE_URL:-https://github.com/doryosi/go-learning-app}
```

Third, commit all source changes before building, verify that the working tree
is clean, and supply the exact commit and build time:

```sh
git status --short

export IMAGE_TAG=v0.1.2
export VCS_REVISION="$(git rev-parse HEAD)"
export BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

docker compose build
```

Finally, compare the embedded revision with Git:

```sh
git rev-parse HEAD

docker image inspect go-learning-app-api:v0.1.2 \
  --format 'version={{index .Config.Labels "org.opencontainers.image.version"}} revision={{index .Config.Labels "org.opencontainers.image.revision"}} created={{index .Config.Labels "org.opencontainers.image.created"}} source={{index .Config.Labels "org.opencontainers.image.source"}}'
```

The two revision values should match. If the working tree contains uncommitted
source changes, the label cannot fully describe the image, which is why clean,
committed builds are important. DOR-25 automates this workflow, and the
later GitHub Actions pipeline will obtain the revision directly from CI.

## Start the local environment

Start both services in the background:

```sh
docker compose up --build --detach
```

The API publishes container port 8080 on host port 8080. The worker publishes
no port because it is a background process rather than a network service.

Inspect their state and logs:

```sh
docker compose ps
docker compose logs api
docker compose logs worker
```

The API state should become `healthy`. The worker should remain `Up` and its
logs should contain `worker started`.

## Verify the server

Check the operational endpoints from the host:

```sh
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
```

Create a user and log in:

```sh
curl -sS -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"learner@example.com","password":"learning-go"}'

curl -sS -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"learner@example.com","password":"learning-go"}'
```

Use the returned token for authenticated project, job, and file-metadata
requests. All data remains in API process memory and disappears when that
container is recreated.

## Recreate, stop, and clean up

Recreate both services from the selected image version:

```sh
docker compose up --detach --force-recreate
```

Stop and remove the containers and Compose network:

```sh
docker compose down
```

Remove the locally built versioned images when they are no longer needed:

```sh
docker image rm go-learning-app-api:v0.1.0 go-learning-app-worker:v0.1.0
```

## Taskfile workflow

Install go-task on macOS from its official Homebrew tap:

```sh
brew install go-task/tap/go-task
```

Run `task` to get a discoverable list of commands. The main Go workflow is:

```sh
task go:build
task go:fmt
task go:check
task go:run:api
task go:run:worker
task go:clean
```

The build task writes the API, worker, and health-check binaries under `bin/`.
That directory contains generated output and is ignored by Git. The two run
tasks are intentionally long-running and stop when you press `Ctrl+C`.

The main Docker workflow is:

```sh
IMAGE_TAG=v0.1.3 task docker:build
IMAGE_TAG=v0.1.3 task docker:up
IMAGE_TAG=v0.1.3 task docker:status
IMAGE_TAG=v0.1.3 task docker:logs -- api
IMAGE_TAG=v0.1.3 task docker:recreate
IMAGE_TAG=v0.1.3 task docker:down
```

`IMAGE_TAG` defaults to `dev`, but an explicit version is preferable when the
image will be inspected or shared. `VCS_REVISION` comes from `git rev-parse
HEAD`, `BUILD_DATE` is generated in UTC, and `SOURCE_URL` defaults to this
GitHub repository. A caller can still override any of these variables.

Task names use namespaces such as `go:*` and `docker:*`, which keeps the command
list readable as the project grows. Composite tasks call smaller tasks rather
than duplicating their shell commands: `go:check` runs tests and vetting,
`docker:build` builds both image targets, and the start/recreate tasks depend on
the common image build.

Task is the single command runner for Go, Docker, and Terraform workflows. The
older Makefile was removed after its Terraform targets were migrated to
`terraform:init`, `terraform:fmt`, `terraform:validate`, and `terraform:plan`.

A later AWS issue will tag the same two image targets with ECR repository
addresses and push them to AWS.
