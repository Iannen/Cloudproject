# Stage 1: Build the Go binary
FROM golang:1.25-alpine AS builder

# Define the build argument for project naming uniformity
ARG BINARY_NAME=cloud-controller
ENV BINARY_NAME=${BINARY_NAME}

WORKDIR /app

# 1. Copy both module files to leverage Docker caching correctly
COPY go.mod go.sum ./
RUN go mod download

# 2. Copy your actual, changing local source files
COPY src/ ./src/

# 3. Perform a fast incremental build utilizing BuildKit cache mounts
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -v -o ${BINARY_NAME} ./src

# Stage 2: Final minimal image
FROM alpine:3.20

ARG BINARY_NAME=cloud-controller
ENV BINARY_NAME=${BINARY_NAME}

WORKDIR /app

# Copy the compiled binary from the builder stage
COPY --from=builder /app/${BINARY_NAME} .

# Run the binary using sh to properly expand the environment variable
CMD ["sh", "-c", "./${BINARY_NAME}"]
