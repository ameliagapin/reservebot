FROM golang:alpine

WORKDIR /app
COPY . .

RUN go build -o /app

EXPOSE 666

ENTRYPOINT ["/app/reservebot", "-token", "$SLACK_TOKEN", "-challenge", "$SLACK_CHALLENGE"]
