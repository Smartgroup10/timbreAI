FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/api ./cmd/api

FROM alpine:3.20

RUN adduser -D -H appuser && apk add --no-cache ca-certificates
USER appuser
WORKDIR /app
COPY --from=build /out/api /app/api
COPY migrations /app/migrations

ENV MIGRATIONS_DIR=/app/migrations
EXPOSE 8080
CMD ["/app/api"]
