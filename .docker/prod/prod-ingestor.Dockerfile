# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY ./backend/go.mod ./backend/go.sum* ./
RUN go mod download
COPY ./backend .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/ingestor ./ingestor

FROM alpine:3.20
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /bin/ingestor /usr/local/bin/ingestor
USER appuser
ENTRYPOINT ["/usr/local/bin/ingestor"]