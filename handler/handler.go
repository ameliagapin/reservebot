package handler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/ameliagapin/reservebot/models"
	"github.com/ameliagapin/reservebot/util"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var (
	actions = map[string]regexp.Regexp{}
)

type Handler struct {
	client       *slack.Client
	reservations *models.Reservations
}

func New(client *slack.Client) *Handler {
	actions["hello"] = *regexp.MustCompile(`hello.+`)
	actions["reserve"] = *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sreserve\s(.+)`)
	// This regex was an attempt to pull all resources in comma separated list in repeating capture groups. It did not work
	//actions["reserve"] = *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sreserve\s([a-zA-Z0-9]+)(?:\,\s([a-zA-Z0-9]+))?`)

	return &Handler{
		client:       client,
		reservations: models.NewReservations(),
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
		case "reserve":
			return h.reserve(ev)
		default:
			h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, I don't know what to do with that request", false))
		}
	}

	return nil
}

func (h *Handler) sayHello(ev *slackevents.AppMentionEvent) error {
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, something went wrong inside of me.", false))
		return err
	}

	h.client.PostMessage(ev.Channel, slack.MsgOptionText("Hello"+u.Name+".", false))
	return nil
}

func (h *Handler) reserve(ev *slackevents.AppMentionEvent) error {
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, something went wrong inside of me.", false))
		return err
	}

	matches := h.getMatches("reserve", ev.Text)
	resources := h.getResourcesFromCommaList(matches[0])
	if len(resources) == 0 {
		msg := fmt.Sprintf("<@%s> you must specify a resource", ev.User)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
		return nil
	}

	success := []string{}
	for _, res := range resources {
		err := h.reservations.Add(res, u)
		if err != nil {
			h.client.PostMessage(ev.Channel, slack.MsgOptionText(err.Error(), false))
			continue
		}
		success = append(success, res)
	}

	for _, res := range success {
		pos, err := h.reservations.GetPosition(res, u)
		if err != nil {
			// This case really should never happen here, as we are only looping through our success cases
			log.Errorf("%+v", err)
			h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, something went wrong inside of me.", false))
			return err
		}
		msg := ""
		switch pos {
		case 0:
			log.Errorf("%s reserved %s, but is currently not in the queue", u, res)
		case 1:
			msg = fmt.Sprintf("<@%s> currently has %s", u.ID, res)
		default:
			msg = fmt.Sprintf("<@%s> is %s in line for %s", u.ID, util.Ordinalize(pos), res)
		}
		h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
	}

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

// getMatches retrieves all capture group values from a given text for regex action
func (h *Handler) getMatches(action, text string) []string {
	ret := []string{}
	r := actions[action]
	matches := r.FindStringSubmatch(text)
	if len(matches) > 1 {
		for _, m := range matches[1:] {
			ret = append(ret, m)
		}
	}
	return ret
}

func (h *Handler) getResourcesFromCommaList(text string) []string {
	ret := []string{}
	split := strings.Split(text, ",")
	for _, s := range split {
		if len(s) > 0 {
			ret = append(ret, strings.Trim(s, " "))
		}
	}
	return ret
}

func (h *Handler) getUser(uid string) (*models.User, error) {
	u, err := h.client.GetUserInfo(uid)
	if err != nil {
		return nil, err
	}
	return &models.User{
		Name: u.Name,
		ID:   u.ID,
	}, nil
}
