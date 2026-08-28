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
IMAGE_TAG=v0.1.0 docker compose build
```

## Start the local environment

Start both services in the background:

```sh
IMAGE_TAG=v0.1.0 docker compose up --build --detach
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
IMAGE_TAG=v0.1.0 docker compose up --detach --force-recreate
```

Stop and remove the containers and Compose network:

```sh
docker compose down
```

Remove the locally built versioned images when they are no longer needed:

```sh
docker image rm go-learning-app-api:v0.1.0 go-learning-app-worker:v0.1.0
```

The next local-development issue wraps these commands in a Taskfile. A later
AWS issue will tag the same two image targets with ECR repository addresses and
push them to AWS.
