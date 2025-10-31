FROM golang:1.25.1-alpine AS build
WORKDIR /src

# Bring in the sibling module so the replace ../crypto-core resolves
COPY services/crypto-core/ ./services/crypto-core/

# Bring in the messages module
COPY services/messages/go.mod services/messages/go.sum ./services/messages/
WORKDIR /src/services/messages
RUN go mod download

# Copy the rest of the source
COPY services/messages/ ./

# Build
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /usr/local/bin/messages ./cmd/messages

FROM alpine:3.20

FROM alpine:3.20
COPY --from=build /usr/local/bin/messages /usr/local/bin/messages
ENTRYPOINT ["messages"]
