FROM golang:1.23-alpine

WORKDIR /app

# Copy only dependency files and download them
# to leverage Docker cache if they don't change
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary
RUN go build -o server .

CMD ["./server"]
