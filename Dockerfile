# Build stage
FROM registry.access.redhat.com/ubi9/go-toolset:latest AS build

# Use the default user's home directory workspace
WORKDIR /opt/app-root/src

# Copy source code (as root to ensure proper permissions)
USER 0
COPY --chown=1001:0 . .

# Switch back to default user
USER 1001

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o release-confidence-score .
RUN strip release-confidence-score

# Runtime stage
FROM registry.access.redhat.com/ubi9/ubi-minimal

WORKDIR /app
RUN chmod +x /app

# Copy the binary from build stage
COPY --from=build /opt/app-root/src/release-confidence-score /app/release-confidence-score

USER 1001

# Run the binary
ENTRYPOINT ["/app/release-confidence-score"]
