# Use the official Go image as a base image
FROM golang:1.22 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy the Go module files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux go build -o go-proxy ./main.go

# Use a minimal image for the final stage
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the built executable from the builder stage
COPY --from=builder /app/go-proxy .

# Copy the web assets and config file
COPY web ./web
COPY data/config.yaml ./data/config.yaml

# Expose the port the application listens on (assuming it's 8080 based on common Go web apps)
# You might need to adjust this based on your application's actual port
EXPOSE 8080

# Command to run the executable
CMD ["./go-proxy"]
