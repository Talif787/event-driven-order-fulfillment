# syntax=docker/dockerfile:1

FROM golang:1.23-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/relay ./cmd/relay
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/projector ./cmd/projector
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/orchestrator ./cmd/orchestrator

FROM gcr.io/distroless/static-debian12:nonroot AS api
WORKDIR /app
COPY --from=build /out/api /app/api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/api"]

FROM gcr.io/distroless/static-debian12:nonroot AS migrate
WORKDIR /app
COPY --from=build /out/migrate /app/migrate
USER nonroot:nonroot
ENTRYPOINT ["/app/migrate"]

FROM gcr.io/distroless/static-debian12:nonroot AS relay
WORKDIR /app
COPY --from=build /out/relay /app/relay
USER nonroot:nonroot
ENTRYPOINT ["/app/relay"]

FROM gcr.io/distroless/static-debian12:nonroot AS projector
WORKDIR /app
COPY --from=build /out/projector /app/projector
USER nonroot:nonroot
ENTRYPOINT ["/app/projector"]

FROM gcr.io/distroless/static-debian12:nonroot AS orchestrator
WORKDIR /app
COPY --from=build /out/orchestrator /app/orchestrator
USER nonroot:nonroot
ENTRYPOINT ["/app/orchestrator"]
