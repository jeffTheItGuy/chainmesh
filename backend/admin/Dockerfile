# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/admin ./admin

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=build /bin/admin /usr/local/bin/admin
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/admin"]
