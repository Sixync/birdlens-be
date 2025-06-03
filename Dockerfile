FROM golang:1.24.3-alpine3.21 as builder

RUN mkdir /app && mkdir /env

COPY . /app

COPY ./env /env

WORKDIR /app

RUN CGO_ENABLED=0 go build -o birdlens-be ./cmd/api && chmod +x /app/birdlens-be

FROM alpine:3.21.3

RUN mkdir /app && mkdir /env;

WORKDIR /app

COPY --from=builder /app/birdlens-be .

COPY --from=builder /env /env

CMD [ "/app/birdlens-be" ]
