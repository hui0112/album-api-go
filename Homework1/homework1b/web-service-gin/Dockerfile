# Use the official Golang image to build the app
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o main .

# Use a tiny image to run the app
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]