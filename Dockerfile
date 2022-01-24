FROM golang:alpine

WORKDIR /app
COPY . .

RUN go build -o /app

EXPOSE 666

ENTRYPOINT ["sh", "-c", "/app/reservebot -token=${SLACK_TOKEN} -challenge=${SLACK_CHALLENGE} -admins=${SLACK_ADMINS}"]
