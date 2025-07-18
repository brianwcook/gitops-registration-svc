# Build stage
FROM registry.access.redhat.com/ubi10-minimal@sha256:4cfec88c16451cc9ce4ba0a8c6109df13d67313a33ff8eb2277d0901b4d81020 AS builder

# Install build dependencies
# hadolint ignore=DL3041
RUN microdnf install -y git ca-certificates golang && microdnf clean all

# Set working directory
WORKDIR /workspace

# Set Go cache to writable location
ENV GOCACHE=/tmp/.cache/go-build
ENV GOMODCACHE=/tmp/go/pkg/mod

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application for native architecture
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o gitops-registration-service \
    cmd/server/main.go

# Final stage
FROM registry.access.redhat.com/ubi10-minimal@sha256:4cfec88c16451cc9ce4ba0a8c6109df13d67313a33ff8eb2277d0901b4d81020

# Install runtime dependencies including user management tools
# hadolint ignore=DL3041
RUN microdnf install -y ca-certificates shadow-utils && microdnf clean all

# Import from builder
COPY --from=builder /workspace/gitops-registration-service /usr/local/bin/

# Create non-root user
RUN useradd -u 65532 -r -g 0 -s /sbin/nologin \
    -c "gitops-registration user" gitops-registration

# Switch to non-root user
USER 65532:0

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/gitops-registration-service", "healthcheck"]

# Run the service
ENTRYPOINT ["/usr/local/bin/gitops-registration-service"] 