FROM golang:1.24.3-alpine3.21 as builder

RUN mkdir /app

COPY . /app

WORKDIR /app

RUN CGO_ENABLED=0 go build -o birdlens-be ./cmd/api && chmod +x /app/birdlens-be

FROM alpine:3.21.3

RUN mkdir /app

WORKDIR /app

COPY --from=builder /app/birdlens-be .

CMD [ "/app/birdlens-be" ]
