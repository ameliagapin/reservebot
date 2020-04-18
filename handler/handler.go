package handler

import (
	"regexp"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var (
	actions = map[string]regexp.Regexp{}
)

type Handler struct {
	client *slack.Client
}

func New(client *slack.Client) *Handler {
	actions["hello"] = *regexp.MustCompile(`hello.+`)

	return &Handler{
		client: client,
	}
}

func (h *Handler) CallbackEvent(event slackevents.EventsAPIEvent) error {
	innerEvent := event.InnerEvent
	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		action := h.getAction(ev.Text)
		switch action {
		case "hello":
			return h.sayHello(ev)
		default:
			h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, I don't know what to do with that request", false))
		}
	}

	return nil
}

func (h *Handler) sayHello(ev *slackevents.AppMentionEvent) error {
	h.client.PostMessage(ev.Channel, slack.MsgOptionText("Yes, hello.", false))
	return nil
}

func (h *Handler) getAction(text string) string {
	for a, r := range actions {
		if r.MatchString(text) {
			return a
		}
	}
	return ""
}
