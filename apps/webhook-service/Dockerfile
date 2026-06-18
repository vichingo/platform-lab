FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook-service .

FROM scratch
COPY --from=builder /app/webhook-service /webhook-service
EXPOSE 8080
ENTRYPOINT ["/webhook-service"]
