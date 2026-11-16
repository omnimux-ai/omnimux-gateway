# Inspiration service

OmniMux inspiration library sidecar: Go + Gin + GORM with SQLite/MySQL/PostgreSQL support. It owns inspiration records and serves media through `/media/*`.

## Build and test

```bash
make test
make lint
make build-linux
make docker-build
```

The Docker build expects the host-built `bin/inspiration-linux` and runs as a distroless non-root image. Runtime configuration is supplied through environment variables by the deployment environment.
