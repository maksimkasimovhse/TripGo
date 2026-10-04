FROM golang:1.27.1 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/trip-service ./cmd/trip-service

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/trip-service /trip-service

USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/trip-service"]