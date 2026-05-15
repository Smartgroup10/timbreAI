FROM golang:1.23-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/voice-agent ./cmd/voice-agent

FROM alpine:3.20
# ffmpeg encode WAV→OGG/Opus para grabaciones (~16x menos tamaño que WAV PCM).
# ~12 MB extra en la imagen final, vale la pena vs ahorro de almacenamiento.
# Si el binario no está, EncodeOpus hace fallback automático a WAV.
RUN adduser -D -H appuser && apk add --no-cache ca-certificates ffmpeg
USER appuser
WORKDIR /app
COPY --from=build /out/voice-agent /app/voice-agent

EXPOSE 8090
CMD ["/app/voice-agent"]
