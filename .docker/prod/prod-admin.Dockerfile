# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /app
COPY ./backend/go.mod ./backend/go.sum* ./
RUN go mod download
COPY ./backend .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/admin ./admin

FROM alpine:3.20
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /bin/admin /usr/local/bin/admin
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/admin"]