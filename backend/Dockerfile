# ---- Builder stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /server ./cmd/server

# ---- Production stage ----
FROM alpine:3.20 AS production

RUN apk add --no-cache ca-certificates=20260413-r0 && \
    addgroup -S app -g 10001 && \
    adduser -S -G app -u 10001 app

WORKDIR /app

COPY --from=builder /server ./server

USER 10001:10001

EXPOSE 8080

CMD ["./server"]
