FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

# Install templ
RUN go install github.com/a-h/templ/cmd/templ@latest

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Generate templ files
RUN templ generate

# Build static binary (no CGO — pure Go SQLite driver)
RUN CGO_ENABLED=0 GOOS=linux go build -o xpbx ./cmd/xpbx

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/xpbx .
COPY --from=builder /app/static ./static

EXPOSE 8080
CMD ["./xpbx"]
