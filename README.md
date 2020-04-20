# reservebot
A Slackbot for reserving shared resources.

# Features

reservebot lets you and your team reserve shared resources and provides a queue for waiting for resources. Currently, reservebot stores reservations in memory so reservations will be lost on restart.

# Running

You will need to set up an application in your Slack admin for `reservebot`.

To run locally, you will need to use a tool to open up your machine to incoming requests. I suggest `ngrok`.

In one window:
```
$ ngrok http 666
```

### Locally on host machine
In a second window:
```
$ go build reservebot.go
$ ./reservebot -token "<YOUR_SLACK_TOKEN>" -challenge "<SLACK_VERIFICATION_TOKEN>"
```

Then in Slack, set up "event subscriptions" for `<ngrok url from your terminal>/events`.

### Docker
```
$ docker build -t reservebot .
$ docker run [-d] -p 666:666 reservebot -token "<YOUR_SLACK_TOKEN>" -challenge "<SLACK_VERIFICATION_TOKEN>"
```

Then in Slack, set up "event subscriptions" for `<ngrok url from your terminal>/events`.

# Commands

#### `@reservebot reserve <resource>`

This will reserve a given resource for the user. If the resource is currently reserved, the user will be placed into the queue. The resource should be an alphanumeric string with no spaces. A comma-separted list can be used to reserve multiple resources.

#### `@reservebot release <resource>`

This will release a given resource. This command must be executed by the person who holds the resource. Upon release, the next person waiting in line will be notified that they now have the resource. The resource should be an alphanumeric string with no spaces. A comma-separted list can be used to reserve multiple resources.

#### `@reservebot status`

This will provide a status of all active resources.

#### `@reservebot status <resource>`

This will provide a status of a given resource.

#### `@reservebot remove me from <resource>`

This will remove the user from the queue for a resource.

#### `@reservebot clear <resource>`
This will clear the queue for a given resource and release it.

#### `@reservebot nuke`
This will clear all reservations and all queues for all resources.
