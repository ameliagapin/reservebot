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

### Helm
The helm installation assumes it will be running on an EKS cluster with an ALB configured.

```
# to install

$ helm install reservebot-release chart --namespace reservebot --set slackToken=<YOUR_SLACK_TOKEN> --set slackVerificationToken=<SLACK_VERIFICATION_TOKEN>

# to upgrade (update)

$ helm upgrade reservebot-release chart --namespace reservebot 

# to upgrade the token values

$ helm upgrade reservebot-release chart --namespace reservebot --set slackToken=<YOUR_SLACK_TOKEN> --set slackVerificationToken=<SLACK_VERIFICATION_TOKEN>

# to find the url of the running service
kubectl -n reservebot get ingresses

# output 
NAME                 HOSTS   ADDRESS                                                                  PORTS   AGE
reservebot-release   *       2b2b5c59-reservebot-reserv-6661-1516092071.us-east-1.elb.amazonaws.com   80      14m
```

# Usage

@reservebot can be used via any channel that it has been added to or via DM. Regardless of where you invoke a command, there is a single reservation system that will be shared.

@reservebot can handle multiple environments or namespaces. A resource is defined as `env|name`. If you omit the environment/namespace, the global environment will be used.

When invoking via DM, the bot will alert other users via DM when necessary. E.g. Releasing a resource will notify the next user that has it.

## Commands

When invoking within a channel, you must @-mention the bot by adding `@reservebot` to the _beginning_ of your command.

#### `reserve <resource>`

This will reserve a given resource for the user. If the resource is currently reserved, the user will be placed into the queue. The resource should be an alphanumeric string with no spaces. A comma-separted list can be used to reserve multiple resources.

#### `release <resource>`

This will release a given resource. This command must be executed by the person who holds the resource. Upon release, the next person waiting in line will be notified that they now have the resource. The resource should be an alphanumeric string with no spaces. A comma-separted list can be used to reserve multiple resources.

#### `status`

This will provide a status of all active resources.

#### `my status`

This will provide a status for all active and waiting resources for the user.

#### `status <resource>`

This will provide a status of a given resource.

#### `remove me from <resource>`

This will remove the user from the queue for a resource.

#### `clear <resource>`
This will clear the queue for a given resource and release it.

#### `<@user>`

This will kick the mentioned user from _all_ resources they are holding. As the user is kicked from each resource, the queue will be advanced to the next user waiting.

#### `nuke`

This will clear all reservations and all queues for all resources. This can only be done from a public channel, not a DM. There is no confirmation, so be careful.
