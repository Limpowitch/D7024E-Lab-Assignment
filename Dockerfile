# syntax=docker/dockerfile:1

# Use a Go version >= what's required in go.mod (1.25.x here)
FROM golang:1.25.1-alpine AS build
WORKDIR /src

# you need git for 'go mod download' when deps are fetched from VCS
RUN apk add --no-cache git

# If you *do* have a go.sum, copy both; otherwise copy only go.mod
COPY go.mod go.sum ./
RUN go env && go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /app/node ./kademlia/cmd/node

FROM alpine:3.20
WORKDIR /app
COPY --from=build /app/node /app/node
RUN chmod +x /app/node
EXPOSE 9999/udp
ENTRYPOINT ["/app/node"]
