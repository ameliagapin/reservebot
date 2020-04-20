FROM golang:alpine

WORKDIR /app
COPY . .

RUN go build -o /app

EXPOSE 666

ENTRYPOINT ["/app/reservebot"]
