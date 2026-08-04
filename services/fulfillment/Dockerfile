# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.23-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/consumer ./cmd/consumer

# ---- api runtime ----
FROM gcr.io/distroless/static-debian12:nonroot AS api
WORKDIR /
COPY --from=build /out/api /api
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/api"]

# ---- migrate runtime ----
FROM gcr.io/distroless/static-debian12:nonroot AS migrate
WORKDIR /
COPY --from=build /out/migrate /migrate
USER nonroot:nonroot
ENTRYPOINT ["/migrate"]

# ---- consumer runtime ----
FROM gcr.io/distroless/static-debian12:nonroot AS consumer
WORKDIR /
COPY --from=build /out/consumer /consumer
USER nonroot:nonroot
ENTRYPOINT ["/consumer"]
