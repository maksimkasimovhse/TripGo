FROM golang:1.27.1 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/maksimkasimovhse/TripGo/internal/infra/telemetry.Version=${VERSION}" -o /out/service ./cmd/service

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/service /service

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/service"]
