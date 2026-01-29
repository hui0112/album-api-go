# Use the official Go image
FROM golang:1.21-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the local code to the container
COPY . .

# Download dependencies (like Gin)
RUN go mod download

# Build the application
RUN go build -o main .

# Expose the port your server uses
EXPOSE 8080

# Run the binary
CMD ["./main"]