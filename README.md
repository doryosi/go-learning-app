# Go SaaS Learning Lab

A SaaS-shaped Go service built progressively to learn AWS. This first milestone
runs entirely on your machine and uses only Go's standard library.

For a detailed explanation of the project structure and the Go concepts used,
read the [Milestone 1 walkthrough](docs/milestone-1-walkthrough.md).

## Milestone 1: local Go service

The current slice provides:

- `GET /healthz`: proves the process is alive.
- `GET /readyz`: proves the process is ready to receive traffic.
- `POST /users` and `POST /login`: creates a local user and bearer session.
- Authenticated project, job, and file-metadata endpoints from DOR-5.
- `PORT` configuration, defaulting to `8080`.
- graceful shutdown on `Ctrl+C` or `SIGTERM`.
- handler tests that do not need a real network port.

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

There is intentionally no Dockerfile, database, cloud SDK, Terraform, or web
framework yet. Application state and sessions are in memory and reset whenever
the server restarts; later milestones replace these boundaries with managed
services.
