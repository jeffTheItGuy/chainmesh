FROM golang:1.25-bookworm
WORKDIR /app

# Install golang-migrate straight to /usr/local/bin so it's usable by any user
RUN GOBIN=/usr/local/bin go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.18.1

# Pre-download modules for faster repeat runs
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY .docker/test/run-tests.sh /run-tests.sh
RUN chmod +x /run-tests.sh

RUN groupadd -r appuser && useradd -r -g appuser -d /app appuser \
    && chown -R appuser:appuser /app
USER appuser