ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o irrigator ./cmd/controller/

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /app/irrigator .
USER 65532:65532
ENTRYPOINT ["/irrigator"]
