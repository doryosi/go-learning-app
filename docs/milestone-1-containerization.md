# Milestone 1: Containerization Walkthrough

This guide explains the DOR-6 changes that turn the Go source code into two
runnable containers:

```text
Source code
   │
   ├── build API binary ────→ small API image
   │                            └── HTTP health check
   │
   └── build worker binary ─→ small worker image
                                └── waits for future queue work
```

## Dockerfile

The `Dockerfile` describes how Docker builds the API and worker images.

### Dockerfile syntax

```dockerfile
# syntax=docker/dockerfile:1
```

This asks Docker to use the current stable Dockerfile syntax.

### Builder stage

```dockerfile
FROM golang:1.24-alpine AS build
WORKDIR /src
```

The first stage contains the Go compiler. Alpine is a relatively small Linux
distribution, `AS build` names this temporary stage, and `WORKDIR` selects the
directory used by subsequent instructions. The compiler image is not included
in either final image.

### Module caching

```dockerfile
COPY go.mod ./
RUN go mod download
```

The module definition is copied before the source code so Docker can reuse its
dependency layer whenever `go.mod` has not changed. There are currently no
third-party dependencies, but this becomes valuable when database and AWS
packages arrive.

### Source code

```dockerfile
COPY cmd ./cmd
COPY internal ./internal
```

Only directories required for compilation enter the build. Documentation, Git
history, and editor configuration are unnecessary in the image.

### Target platforms

```dockerfile
ARG TARGETOS
ARG TARGETARCH
```

Docker supplies these values during platform-aware builds. Examples include
`linux/arm64` and `linux/amd64`. This prepares the layout for ECR and EKS, where
the target CPU architecture might differ from the developer's laptop.

### Compiled binaries

The builder creates three files:

```text
/out/api
/out/worker
/out/healthcheck
```

Important compilation options include:

- `CGO_ENABLED=0` removes dependencies on system C libraries so the programs
  can run in an empty image.
- `GOOS` and `GOARCH` select the target operating system and CPU architecture.
- `-trimpath` removes local filesystem paths from the binary.
- `-ldflags="-s -w"` removes debugging symbols to reduce binary size.

### API runtime image

```dockerfile
FROM scratch AS api
COPY --from=build /out/api /api
COPY --from=build /out/healthcheck /healthcheck
```

`scratch` is an empty base image. It contains no shell, package manager, Go
compiler, Linux utilities, or unnecessary libraries. Only the two copied
binaries exist in the final API image. This reduces both image size and attack
surface.

```dockerfile
USER 65532:65532
```

The process runs with numeric user and group ID `65532`, rather than as root. A
numeric identity works even though a scratch image has no `/etc/passwd` file.
Non-root execution limits what a compromised process can do inside the
container.

```dockerfile
EXPOSE 8080
```

This documents the port used by the API. It does not publish the port to the
host; Compose performs that mapping.

### API health check

```dockerfile
HEALTHCHECK --interval=10s --timeout=2s --start-period=3s --retries=3 \
    CMD ["/healthcheck", "http://127.0.0.1:8080/healthz"]
```

Docker periodically runs the custom health-check binary:

- `--interval=10s` checks every ten seconds.
- `--timeout=2s` limits each attempt to two seconds.
- `--start-period=3s` provides a short startup grace period.
- `--retries=3` requires three consecutive failures before the container is
  declared unhealthy.

A scratch image does not contain `curl`, `wget`, or a shell, which is why the
project includes its own small health-check program.

```dockerfile
ENTRYPOINT ["/api"]
```

Docker runs `/api` when the image starts. The JSON-array form makes the API the
direct application process, allowing it to receive signals such as `SIGTERM`.

### Worker runtime image

```dockerfile
FROM scratch AS worker
COPY --from=build /out/worker /worker
USER 65532:65532
ENTRYPOINT ["/worker"]
```

This produces a separate image containing only the worker binary. The API and
worker use the same builder but have different final targets and entry points.
That maps naturally to separate ECR images and EKS workloads later.

## Docker Compose

`compose.yml` defines how the two containers run together locally.

### API service

```yaml
services:
  api:
    build:
      context: .
      target: api
```

`context: .` sends the project directory to Docker, while `target: api` selects
the API stage from the multi-stage Dockerfile.

```yaml
environment:
  PORT: "8080"

ports:
  - "8080:8080"
```

The environment variable configures the port inside the container. The port
mapping sends traffic from `localhost:8080` to port `8080` in the API container.

### Worker service

```yaml
worker:
  build:
    context: .
    target: worker
```

The worker selects the other Dockerfile target. It exposes no port because it is
a background process rather than an HTTP server.

Both services use:

```yaml
init: true
```

Docker places a small init process in front of each Go program. It helps forward
signals and clean up orphaned child processes.

### Process-memory boundary

The containers do not share Go memory:

```text
API container memory    Worker container memory
      jobs map       ≠       separate process
```

A job placed in the API's map is invisible to the worker. A real queue such as
SQS is required for cross-process communication and arrives in milestone 3.

## Docker build context

`.dockerignore` controls which files Docker can see during a build. It excludes
Git data, local environment files, editor configuration, build output, and
documentation.

This provides three benefits:

1. Smaller build contexts transfer faster.
2. Local or sensitive files such as `.env` do not enter the build.
3. Unrelated changes do not invalidate Docker's build cache.

`.gitignore` and `.dockerignore` have distinct purposes: the first controls what
Git tracks, while the second controls what Docker receives.

## Health-check program

`cmd/healthcheck/main.go` is compiled into the API image. It expects one URL as
its command-line argument, creates an HTTP client with a 1.5-second timeout, and
sends a request to that URL.

It treats status codes from 200 through 299 as healthy and communicates the
result to Docker with process exit codes:

- `0` means healthy.
- `1` means the endpoint is unhealthy.
- `2` means the command was invoked incorrectly.

The response body is closed with `defer` so network resources are released even
though the body content is not needed.

## Worker executable

`cmd/worker/main.go` creates a context cancelled by `Ctrl+C` or `SIGTERM`, logs
the worker lifecycle, and delegates reusable behavior to `internal/worker`.
Keeping the entry point small separates process concerns—signals, logging, and
exit codes—from worker behavior.

The current implementation waits for cancellation:

```go
func Run(ctx context.Context) error {
    <-ctx.Done()
    return nil
}
```

The receive expression `<-ctx.Done()` blocks until the context is cancelled.
The worker therefore starts, stays alive, and exits cleanly when Docker stops
it. It is a lifecycle scaffold for later SQS polling and job processing.

## Worker test

`internal/worker/worker_test.go` starts `Run` in a goroutine, cancels its
context, and waits for the function to return. A one-second timeout prevents the
test from hanging forever if shutdown is broken.

This verifies that Docker and Kubernetes can stop the worker gracefully rather
than killing it forcibly.

## Taskfile commands

The Taskfile wraps the common container lifecycle commands using Docker
Compose's current CLI plugin form:

```sh
task docker:build   # Build both images
task docker:up      # Build and start both services
task docker:down    # Stop and remove containers and their network
```

The worker can also be run directly with:

```sh
task go:run:worker
```

## Verification results

The implementation was verified with:

- All Go tests
- The Go race detector
- `go vet`
- Docker builds for both targets
- API health and readiness requests
- Docker's API health status
- Non-root identities for both containers
- Graceful shutdown of both processes

The resulting scratch-based images were approximately:

- API: 4.5 MB
- Worker: 0.8 MB

The worker is smaller because it only contains lifecycle behavior. The API also
contains HTTP, JSON, authentication, local storage, and health-check behavior.
