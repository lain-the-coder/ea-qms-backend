# ---------------------------------------------------
# Stage 1: Build Stage (Compiler & Dependencies)
# ---------------------------------------------------
FROM golang:1.25-alpine AS builder
WORKDIR /app

# Step A: Cache Go modules layer
COPY go.mod go.sum ./
RUN go mod download

# Step B: Copy source and cross-compile a static binary for the target
# architecture. buildx sets TARGETARCH per platform, so one Dockerfile
# produces both amd64 and arm64 without emulation.
COPY . .
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH \
    go build -ldflags="-w -s" -o server .

# ---------------------------------------------------
# Stage 2: Final Runtime Stage (Ultra-lean image)
# ---------------------------------------------------
FROM alpine:latest
WORKDIR /app

# Copy only the compiled binary from the builder stage
COPY --from=builder /app/server .

EXPOSE 1304
CMD ["./server"]
