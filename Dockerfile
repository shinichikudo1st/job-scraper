FROM golang:1.22-alpine AS builder
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o /bin/job-scraper ./cmd/job-scraper

FROM alpine:3.19
WORKDIR /app

RUN addgroup -S app && adduser -S -G app app && apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/job-scraper /app/job-scraper

EXPOSE 8080
USER app

ENTRYPOINT ["/app/job-scraper"]
