# syntax=docker/dockerfile:1

### Build stage
FROM golang:1.23.5-alpine AS build
WORKDIR /src

# Cache deps (no go.sum in this project yet)
COPY go.mod ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /out/node ./cmd/main

### Runtime stage
FROM alpine:3.20
ENV PORT=8080
EXPOSE 8080
COPY --from=build /out/node /usr/local/bin/node
ENTRYPOINT ["usr/local/bin/node"]
