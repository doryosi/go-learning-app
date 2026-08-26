# Go SaaS Learning Lab

A SaaS-shaped Go service built progressively to learn AWS. This first milestone
runs entirely on your machine and uses only Go's standard library.

For a detailed explanation of the project structure and the Go concepts used,
read the [Milestone 1 walkthrough](docs/milestone-1-walkthrough.md).
For an explanation of the Dockerfile, Compose environment, health check, and
worker image, read the [containerization walkthrough](docs/milestone-1-containerization.md).
The AWS networking work is explained in the
[milestone 2 networking walkthrough](docs/milestone-2-networking.md).

## Milestone 1: local Go service

The current slice provides:

- `GET /healthz`: proves the process is alive.
- `GET /readyz`: proves the process is ready to receive traffic.
- `POST /users` and `POST /login`: creates a local user and bearer session.
- Authenticated project, job, and file-metadata endpoints from DOR-5.
- `PORT` configuration, defaulting to `8080`.
- graceful shutdown on `Ctrl+C` or `SIGTERM`.
- handler tests that do not need a real network port.
- separate, non-root container images for the API and worker.
- a local Docker Compose environment with an API health check.

These operational endpoints look small, but later milestones will reuse them for
Docker health checks, ALB target health checks, and Kubernetes probes.

### Prerequisite

Install Go 1.22 or newer, then confirm it is available:

```sh
go version
```

### Run it

```sh
make run
```

In a second terminal:

```sh
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
```

Create a user, log in, then send the returned token as
`Authorization: Bearer <token>`:

```sh
curl -sS -X POST http://localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"learner@example.com","password":"learning-go"}'

curl -sS -X POST http://localhost:8080/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"learner@example.com","password":"learning-go"}'
```

Try configuration without changing code:

```sh
PORT=9090 make run
```

Stop the server with `Ctrl+C`; the log should show that it received the shutdown
signal and let in-flight requests finish before exiting.

### Run it with Docker

With Docker Engine or Docker Desktop running:

```sh
make docker-up
```

This builds two small images from the multi-stage `Dockerfile`: `api` exposes
port 8080 and has a container health check, while `worker` exercises the
background-process lifecycle. Both run as an unprivileged user. Stop and remove
the containers with:

```sh
make docker-down
```

The worker intentionally does not consume the API's in-memory jobs. Separate
containers cannot share process memory; real cross-process job delivery arrives
with SQS in milestone 3.

### Verify it

```sh
make check
```

`go test` uses `httptest`, which calls the HTTP handler directly. This keeps the
test fast and deterministic. `go vet` catches common correctness mistakes that
the compiler accepts.

## What to notice

`cmd/api` is the executable entry point. `internal/httpapi` holds reusable HTTP
behavior and cannot be imported by unrelated Go modules. Keeping route behavior
outside `main` makes it easy to test and keeps startup/shutdown concerns separate
from request handling.

Liveness and readiness deliberately start with similar behavior. Their meanings
will diverge when dependencies arrive: liveness should normally stay healthy
while readiness can fail if the application cannot safely serve traffic.

## Milestone boundary

There is intentionally no database, cloud SDK, Terraform, or web framework yet.
Application state and sessions are in memory and reset whenever the API
container restarts; later milestones replace these boundaries with managed
services.

## Milestone 2: AWS network foundation

The first AWS slice lives in `infra/terraform/network`. It defines a VPC,
public and private subnets in multiple Availability Zones, route tables, an
Internet Gateway, optional cost-aware NAT Gateways, and security groups for the
future ALB and private application compute.

The default NAT mode is `none`, so merely using the example configuration does
not request a charged NAT Gateway. Terraform does not create anything until an
authenticated user deliberately runs `terraform apply`. Start with the
[network module guide](infra/terraform/network/README.md), inspect the plan,
and destroy experiments promptly.
