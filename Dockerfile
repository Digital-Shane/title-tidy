FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .

RUN go build -o title-tidy .

FROM alpine:3.22
COPY --from=builder /app/title-tidy /usr/local/bin/title-tidy

ENTRYPOINT ["/bin/sh"]