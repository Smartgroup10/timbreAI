FROM golang:1.22-alpine AS build

WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.20

RUN adduser -D -H appuser
USER appuser
WORKDIR /app
COPY --from=build /out/api /app/api

EXPOSE 8080
CMD ["/app/api"]

