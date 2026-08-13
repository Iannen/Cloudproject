FROM golang:1.25-alpine AS builder

ARG BINARY_NAME=cloud-controller
ENV BINARY_NAME=${BINARY_NAME}

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY src/ ./src/

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=linux go build -v -o ${BINARY_NAME} ./src

FROM alpine:3.20

ARG BINARY_NAME=cloud-controller
ENV BINARY_NAME=${BINARY_NAME}

RUN apk add --no-cache docker-cli docker-cli-compose tailscale
WORKDIR /app

COPY --from=builder /app/${BINARY_NAME} .

CMD ["sh", "-c", "./${BINARY_NAME}"]
