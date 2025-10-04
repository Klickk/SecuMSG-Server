# --- Build stage ---
FROM golang:1.25.1-alpine AS build
WORKDIR /src


# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /usr/local/bin/auth ./cmd/auth

FROM alpine:3.20

COPY --from=build /usr/local/bin/auth /usr/local/bin/auth

EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/auth"]
