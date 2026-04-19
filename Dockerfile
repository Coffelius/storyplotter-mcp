# syntax=docker/dockerfile:1.6

FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/storyplotter-mcp ./cmd/storyplotter-mcp

FROM alpine:3.20
RUN adduser -D -u 10001 app && apk add --no-cache ca-certificates
USER app
WORKDIR /home/app
COPY --from=builder /out/storyplotter-mcp /usr/local/bin/storyplotter-mcp
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/storyplotter-mcp"]
CMD ["-mode=http", "-addr=:8080"]
