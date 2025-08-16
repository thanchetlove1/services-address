# Stage 1: Build Go application (sử dụng pre-built libpostal)
FROM golang:1.21-bullseye AS builder

# Install CGO dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc pkg-config \
 && rm -rf /var/lib/apt/lists/*

# Copy libpostal từ pre-built image
COPY --from=libpostal-base:latest /usr/local/lib/libpostal* /usr/local/lib/
COPY --from=libpostal-base:latest /usr/local/include/libpostal/ /usr/local/include/libpostal/
COPY --from=libpostal-base:latest /usr/local/lib/pkgconfig/libpostal.pc /usr/local/lib/pkgconfig/

# Update library cache
RUN ldconfig

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# copy source
COPY . .

# build binary với CGO cho libpostal
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w" -a -o main ./cmd/api

# Stage 2: Runtime image
FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates wget \
 && rm -rf /var/lib/apt/lists/*

# Copy libpostal libraries và data từ pre-built image
COPY --from=libpostal-base:latest /usr/local/lib/libpostal* /usr/local/lib/
COPY --from=libpostal-base:latest /usr/local/include/libpostal/ /usr/local/include/libpostal/
COPY --from=libpostal-base:latest /usr/local/share/libpostal/ /usr/local/share/libpostal/

# Set LIBPOSTAL_DATA_DIR environment variable
ENV LIBPOSTAL_DATA_DIR=/usr/local/share/libpostal

# update ld cache
RUN ldconfig

WORKDIR /root/
COPY --from=builder /app/main .
COPY --from=builder /app/config ./config

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./main"]
