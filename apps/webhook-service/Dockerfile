FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o webhook-service .

FROM scratch
COPY --from=builder /app/webhook-service /webhook-service
EXPOSE 8080
ENTRYPOINT ["/webhook-service"]
