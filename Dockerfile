# Use the official Golang image as the builder
FROM golang:1.20-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod ./

# Download dependencies (if any)
RUN go mod download

# Copy the source code
COPY main.go ./

# Build the Go application
RUN go build -o kubeflow-dashboard

# Use a minimal image for the final stage
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/kubeflow-dashboard .

# Expose port 8887 to the outside world
EXPOSE 8887

# Command to run the executable
CMD ["./kubeflow-dashboard"] 