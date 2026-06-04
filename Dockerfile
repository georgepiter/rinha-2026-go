FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/build_index ./cmd/build_index
ADD https://raw.githubusercontent.com/zanfranceschi/rinha-de-backend-2026/main/resources/references.json.gz /app/references.json.gz
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o build_index cmd/build_index/main.go
RUN /app/build_index /app/references.json.gz /app/index.bin
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o api cmd/api/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/api .
COPY --from=builder /app/index.bin .
EXPOSE 9999
CMD ["./api"]
