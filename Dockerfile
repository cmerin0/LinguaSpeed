# Stage 1 — builder
# Uses the full Go toolchain to compile the server binary with CGO disabled so
# the output is a fully static binary that requires no C runtime at execution time.
FROM golang:1.26.7 AS builder

WORKDIR /build

# Copy dependency manifests first so that `go mod download` is cached as its own
# layer. The module cache is only invalidated when go.mod or go.sum changes, not
# every time source code changes — this keeps incremental builds fast.
COPY go.mod go.sum ./
RUN go mod download

# Copy all remaining source and compile. CGO_ENABLED=0 produces a static binary;
# GOOS=linux targets the container OS regardless of the host platform.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o linguaspeed ./cmd/server

# Stage 2 — runtime
# Uses Google's distroless static image: no shell, no package manager, no libc.
# Distroless drastically reduces the attack surface — there is nothing in the
# image except the binary and its minimum OS dependencies (CA certs, tzdata).
FROM gcr.io/distroless/static-debian12

# Run as the built-in non-root user that distroless provides. Running as root
# inside a container is unnecessary and violates the principle of least privilege.
USER nonroot:nonroot

COPY --from=builder /build/linguaspeed /linguaspeed

ENTRYPOINT ["/linguaspeed"]
