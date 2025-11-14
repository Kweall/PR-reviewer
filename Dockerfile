FROM golang:1.24.4 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /PR-reviewer ./cmd/server


FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /PR-reviewer /PR-reviewer
COPY migrations.sql /migrations.sql
EXPOSE 8080
ENTRYPOINT ["/PR-reviewer"]