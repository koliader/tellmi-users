# Build stage
FROM golang:1.26.5-alpine3.24 AS builder
WORKDIR /app

# Copy the sdk first (matches ../tellmi-sdk relative path from go.mod's replace directive)
COPY tellmi-sdk /tellmi-sdk

# Copy go.mod/go.sum first for caching, then download
COPY tellmi-users/go.mod tellmi-users/go.sum ./
RUN go mod download

# Now copy the rest of the app source
COPY tellmi-users/. .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o main cmd/main.go

RUN apk add --no-cache curl
RUN curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar -xzv
RUN mv migrate /usr/local/bin/migrate

# Run stage
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
RUN adduser -D -u 10001 appuser
WORKDIR /app
COPY --from=builder /app/main .
COPY --from=builder /usr/local/bin/migrate /usr/local/bin/migrate
COPY --from=builder /app/internal/store/db/migration ./migration
USER appuser
ENV ENVIRONMENT=docker
EXPOSE 8081
CMD ["/app/main"]
