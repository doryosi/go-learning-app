# Milestone 1: Go SaaS API Walkthrough

This guide explains how the local Go SaaS API is organized, how a request moves
through it, and why the implementation uses its current boundaries. It is a
learning implementation rather than a production authentication system: all
state is held in memory, sessions do not expire, and the password hashing must
be replaced with Argon2id or bcrypt before production use.

## Project layout

```text
go-learning-app/
├── cmd/api/main.go                  Program startup and shutdown
├── internal/config/config.go        Environment configuration
├── internal/config/config_test.go   Configuration tests
├── internal/httpapi/handler.go      Routes and application behavior
├── internal/httpapi/handler_test.go HTTP behavior tests
├── go.mod                           Go module definition
├── Makefile                         Development commands
└── README.md                        Setup and usage
```

Each package has one primary responsibility:

- `cmd/api` turns the packages into an executable program.
- `internal/config` reads and validates runtime configuration.
- `internal/httpapi` implements the HTTP boundary and local application state.
- `_test.go` files test the package beside them.

Keeping startup separate from HTTP behavior lets tests call the handler without
opening a real network port.

## The module

`go.mod` identifies the project as a Go module:

```go
module example.com/go-learning-app

go 1.22
```

The module path becomes the prefix for internal imports, for example:

```go
import "example.com/go-learning-app/internal/httpapi"
```

The project currently uses only the Go standard library.

## Starting the program

`cmd/api/main.go` belongs to the special `main` package and contains the entry
point:

```go
func main() {
    if err := run(); err != nil {
        slog.Error("server stopped", "error", err)
        os.Exit(1)
    }
}
```

The actual work lives in `run()` because `main()` cannot return an error. This
keeps ordinary error handling available during startup.

Configuration is loaded first:

```go
cfg, err := config.Load()
if err != nil {
    return err
}
```

Go functions can return multiple values. Here they are the loaded `Config` and
an `error`. A `nil` error means success.

The HTTP server is then constructed with a struct literal:

```go
server := &http.Server{
    Addr:              cfg.Address,
    Handler:           httpapi.NewHandler(),
    ReadHeaderTimeout: cfg.ReadTimeout,
    WriteTimeout:      cfg.WriteTimeout,
}
```

The address selects the listening port, the handler receives requests, and the
timeouts protect the process from stalled clients.

### Goroutines, channels, and shutdown

`ListenAndServe` blocks while the server is running, so it is launched in a
goroutine:

```go
serverErr := make(chan error, 1)
go func() {
    serverErr <- server.ListenAndServe()
}()
```

A goroutine is lightweight concurrent work. A channel carries the server's
eventual error back to the main flow.

At the same time, `signal.NotifyContext` watches for `Ctrl+C` and `SIGTERM`.
The `select` statement waits until either the server stops or the operating
system requests shutdown:

```go
select {
case err := <-serverErr:
    // The server stopped or failed.
case <-ctx.Done():
    // A shutdown signal arrived.
}
```

Finally, `server.Shutdown` stops accepting requests and gives active requests a
limited amount of time to finish. This behavior will later matter in Docker,
EC2, and Kubernetes.

## Configuration

`internal/config/config.go` groups settings into a struct:

```go
type Config struct {
    Address         string
    ReadTimeout     time.Duration
    WriteTimeout    time.Duration
    ShutdownTimeout time.Duration
}
```

`Load()` reads `PORT`, defaults to `8080`, converts it from text to an integer,
and verifies that it falls between 1 and 65535. Keeping this work in its own
package makes configuration independently testable and provides a natural home
for future database, AWS, and logging settings.

## Building the HTTP handler

`internal/httpapi/handler.go` constructs an `http.Handler`. That interface is
the standard contract for anything capable of serving an HTTP request.

```go
func NewHandler() http.Handler {
    return NewHandlerWithLogger(slog.Default())
}
```

The second constructor accepts a logger. Normal startup uses the default
logger, while tests inject a logger that discards output. Passing a dependency
in this way is a simple form of dependency injection.

### Local state

The `API` struct owns the milestone's in-memory data:

```go
type API struct {
    mu             sync.RWMutex
    users          map[string]userRecord
    emails, tokens map[string]string
    projects       map[string]Project
    jobs           map[string]Job
    files           map[string]File
    logger          *slog.Logger
}
```

A map stores key-value pairs. For example, `map[string]Project` maps a project
ID to its `Project` value. This state disappears when the process restarts.

Go's HTTP server handles requests concurrently, and ordinary maps are unsafe
for simultaneous writes. `sync.RWMutex` protects them:

- `Lock` and `Unlock` provide exclusive access for a write.
- `RLock` and `RUnlock` allow multiple readers but exclude writers.

Later milestones can replace these maps with repository interfaces backed by
PostgreSQL, Redis, SQS, and S3.

## Data types and JSON

A resource is represented by a struct:

```go
type Project struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    OwnerID   string    `json:"owner_id"`
    CreatedAt time.Time `json:"created_at"`
}
```

The backtick expressions are struct tags. They tell the JSON encoder to render
`OwnerID` as `owner_id`, for example. Capitalized fields are exported from the
package and are visible to the JSON encoder.

The private `userRecord` embeds the public `User` and adds password data. User
creation returns only `record.User`, preventing the salt and hash from being
included in the response.

## Routes and middleware

Routes are registered with Go 1.22 method-and-path patterns:

```go
mux.HandleFunc("POST /users", a.createUser)
mux.Handle("GET /projects", a.auth(http.HandlerFunc(a.listProjects)))
```

The protected route is wrapped by authentication. The full request path is:

```text
request → logging middleware → authentication middleware → route → response
```

Middleware centralizes behavior that many routes share. It prevents every route
from having to repeat authorization and logging code.

### Health checks

`GET /healthz` answers whether the process is alive. `GET /readyz` answers
whether it can safely receive traffic. Their behavior is currently similar
because there are no external dependencies. Readiness can become stricter when
the application gains required services.

## Reading JSON safely

`decodeJSON` applies the same parsing rules to every JSON request:

1. `http.MaxBytesReader` limits a body to one MiB.
2. `json.Decoder` converts JSON into a Go value.
3. `DisallowUnknownFields` catches misspelled or unexpected properties.
4. A second decode ensures the body contains exactly one JSON value.

Handlers decode into a small local struct when a request shape is used only in
that function:

```go
var in struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}
```

## Users and login

User creation normalizes the email, performs basic validation, and generates a
random user ID and salt with `crypto/rand`. Passwords are not stored directly.
The current local implementation stores a salted SHA-256 hash.

SHA-256 is too fast for production password storage. It is retained only to
keep this first local slice dependency-free; use Argon2id or bcrypt before
deploying real authentication.

Duplicate emails return `409 Conflict`. Login looks up the user, hashes the
submitted password with the stored salt, and uses `subtle.ConstantTimeCompare`
to compare hashes without an ordinary timing-sensitive string comparison.

Successful login creates a random token. Protected calls send it as:

```http
Authorization: Bearer <token>
```

Current sessions are intentionally limited: they live in memory, do not expire,
and have no logout or revocation flow.

## Authentication context

The authentication middleware extracts the bearer token and resolves its user.
If it is invalid, the request stops with `401 Unauthorized`. Otherwise, the
user ID is placed in the request's `context.Context` and the next handler runs.

Handlers call `currentUser(r)` to retrieve that trusted identity. They do not
accept an owner ID from client JSON, which prevents a client from assigning a
resource to another user.

## Projects, jobs, and files

`POST /projects` creates an owned project, while `GET /projects` returns only
the current user's projects.

`POST /jobs` creates a job with the status `queued` and returns `202 Accepted`.
That status says the work was accepted but has not necessarily completed. A
later worker and SQS milestone can process these jobs. `GET /jobs` lists the
user's jobs, and `GET /jobs/{id}` uses `r.PathValue("id")` to retrieve one.

The file routes currently store metadata, not file bytes. That provides a real
resource and ownership model without preempting the later S3 milestone.

When a resource belongs to another user, the API returns the same `404` used
for a missing resource. This avoids revealing that another user's resource
exists.

## Responses and logging

`writeJSON` sets the content type, writes the status, and encodes a value.
`writeError` builds on it so errors consistently look like:

```json
{"error":"description"}
```

The logging middleware records the method, path, status, and duration. Because
`http.ResponseWriter` does not expose a response status afterward,
`statusWriter` wraps it and remembers calls to `WriteHeader`.

## Tests

`internal/httpapi/handler_test.go` uses `httptest.NewRequest` and
`httptest.NewRecorder`. The handler is invoked directly, so tests do not need a
network port.

The tests demonstrate common Go patterns:

- Table-driven health tests run the same assertions for multiple cases.
- The workflow test registers a user, logs in, and calls protected resources.
- The authentication test verifies that a protected route rejects a missing
  token.
- Configuration tests use `t.Setenv`, which restores the environment after the
  test.

Run the complete verification suite with:

```sh
make check
go test -race ./...
```

The race detector is particularly valuable here because the HTTP layer and its
in-memory maps are concurrent.

## Recommended next improvements

Before treating the service as production-ready:

1. Split `handler.go` into focused user, project, job, file, and middleware
   files within the same package.
2. Introduce repository interfaces so handlers do not manipulate maps directly.
3. Replace password hashing with Argon2id or bcrypt.
4. Add session expiration, revocation, rate limiting, and email verification.
5. Sort list results instead of relying on unspecified map iteration order.
6. Add tests for invalid JSON, duplicate users, incorrect passwords, ownership,
   and individual job and file retrieval.
7. Propagate random-ID generation errors instead of representing failure with an
   empty string.

The current design is a useful educational foundation: small enough to trace in
one sitting, but substantial enough to demonstrate packages, structs,
interfaces, errors, contexts, goroutines, channels, middleware, concurrency,
JSON, and testing through a working application.
