# Use an official Golang runtime as the parent image
FROM golang:latest

# Install awscli for AWS interactions
RUN apt-get update && \
    apt-get install -y awscli && \
    rm -rf /var/lib/apt/lists/*

# Set the working directory inside the container to /app
WORKDIR /app

# Copy the local source files into the container's /app directory
COPY . .

# Build the Go application
RUN go build -o translator

# Expose port 7008 for the application
EXPOSE 7008

# Command to run the Go application
CMD ["./translator"]
