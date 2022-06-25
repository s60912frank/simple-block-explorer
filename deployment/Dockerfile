FROM golang:1.17-buster as builder

COPY ../pkg /app
WORKDIR /app
RUN go get && go build -o /app/build/main main.go

FROM busybox:1.34.0-glibc

COPY --from=builder /app/build/ /app
COPY --from=builder /etc/ssl/certs/ /etc/ssl/certs/

WORKDIR /app

RUN chmod +x main

CMD ["./main"]