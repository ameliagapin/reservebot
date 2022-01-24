package handler

import (
	"fmt"
	"strings"
	"time"

	"github.com/ameliagapin/reservebot/data"
	e "github.com/ameliagapin/reservebot/err"
	"github.com/ameliagapin/reservebot/models"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type Handler struct {
	client *slack.Client
	data   data.Manager

	reqEnv bool
	admins []string
}

type EventAction struct {
	Event  *slackevents.MessageEvent
	Action string
}

func New(client *slack.Client, data data.Manager, reqEnv bool, admins []string) *Handler {
	return &Handler{
		client: client,
		data:   data,
		reqEnv: reqEnv,
		admins: admins,
	}
}

func (h *Handler) CallbackEvent(event slackevents.EventsAPIEvent) error {
	// First, we normalize the incoming event
	var ea *EventAction
	innerEvent := event.InnerEvent
	switch ev := innerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		ea = &EventAction{
			Event: &slackevents.MessageEvent{
				Type:            ev.Type,
				User:            ev.User,
				Text:            ev.Text,
				TimeStamp:       ev.TimeStamp,
				ThreadTimeStamp: ev.ThreadTimeStamp,
				Channel:         ev.Channel,
				EventTimeStamp:  ev.EventTimeStamp,
				UserTeam:        ev.UserTeam,
				SourceTeam:      ev.SourceTeam,
			},
		}
	case *slackevents.MessageEvent:
		if h.shouldHandle(ev) {
			ea = &EventAction{
				Event: ev,
			}
		}
	}
	if ea == nil {
		return nil
	}

	// Now we determine what to do with it
	ea.Action = h.getAction(ea.Event.Text)
	switch ea.Action {
	case "hello":
		return h.sayHello(ea)
	case "reserve", "reserve_dm":
		return h.reserve(ea)
	case "release", "release_dm":
		return h.release(ea)
	case "removeme", "removeme_dm":
		return h.removeme(ea)
	case "removeresource", "removeresource_dm":
		return h.removeresource(ea)
	case "clear", "clear_dm":
		return h.clear(ea)
	case "kick", "kick_empty", "kick_nonuser", "kick_dm":
		return h.kick(ea)
	case "nuke":
		return h.nuke(ea)
	case "nuke_dm":
		return h.reply(ea, "You must perform a nuke action from a public channel", false)
	case "all_status", "all_status_dm", "my_status", "my_status_dm":
		return h.allStatus(ea)
	case "single_status", "single_status_dm":
		return h.singleStatus(ea)
	case "prune", "prune_dm":
		return h.prune(ea)
	case "help", "help_dm":
		return h.help(ea)
	default:
		return h.reply(ea, "I'm sorry, I don't know what to do with that request", false)
	}
}

func (h *Handler) shouldHandle(ev *slackevents.MessageEvent) bool {
	if ev.BotID != "" {
		return false
	}
	if ev.ChannelType != "im" {
		return false
	}

	return true
}

func (h *Handler) sayHello(ea *EventAction) error {
	ev := ea.Event
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.errorReply(ev.Channel, "")
		return err
	}

	h.client.PostMessage(ev.Channel, slack.MsgOptionText("Hello"+u.Name+".", false))
	return nil
}

func (h *Handler) getCurrentResText(resource *models.Resource, mention bool) (string, error) {
	q, err := h.data.GetQueueForResource(resource.Name, resource.Env)
	if err != nil {
		return "", err
	}

	msg := ""
	queue := []string{}

	switch len(q.Reservations) {
	case 0:
		msg = fmt.Sprintf("`%s` is free", resource)
	case 1:
		user := h.getUserDisplayWithDuration(q.Reservations[0], mention)
		msg = fmt.Sprintf("`%s` is currently reserved by %s", resource, user)
	default:
		verb := "is"
		for _, next := range q.Reservations[1:] {
			queue = append(queue, h.getUserDisplayWithDuration(next, false))
		}
		if len(queue) > 1 {
			verb = "are"
		}
		user := h.getUserDisplayWithDuration(q.Reservations[0], mention)
		msg = fmt.Sprintf("`%s` is currently reserved by %s. %s %s waiting.", resource, user, strings.Join(queue, ", "), verb)
	}

	return msg, nil
}

func (h *Handler) getUserDisplay(user *models.User, mention bool) string {
	ret := fmt.Sprintf("*%s*", user.Name)
	if mention {
		ret = fmt.Sprintf("<@%s>", user.ID)
	}
	return ret
}

func (h *Handler) getUserDisplayWithDuration(reservation *models.Reservation, mention bool) string {
	user := reservation.User
	dur := getDuration(reservation.Time)

	ret := fmt.Sprintf("*%s* (%s)", user.Name, dur)
	if mention {
		ret = fmt.Sprintf("<@%s> (%s)", user.ID, dur)
	}
	return ret
}

func getDuration(t time.Time) string {
	duration := time.Since(t).Round(time.Minute)

	if duration < 1 {
		return "0m"
	}

	d := duration.String()

	return d[:len(d)-2]
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

func (h *Handler) getResourcesFromCommaList(text string) ([]*models.Resource, error) {
	ret := []*models.Resource{}
	split := strings.Split(text, ",")
	for _, s := range split {
		if len(s) > 0 {
			r, err := h.parseResource(strings.Trim(s, " `"))
			if err != nil {
				return nil, err
			}
			if r != nil {
				ret = append(ret, r)
			}
		}
	}
	if len(ret) == 0 {
		return nil, e.NoResourceProvided
	}

	return ret, nil
}

func (h *Handler) parseResource(text string) (*models.Resource, error) {
	split := strings.Split(text, "|")
	switch len(split) {
	case 1:
		if h.reqEnv {
			return nil, e.InvalidResourceFormat
		}
		return &models.Resource{
			Name: split[0],
		}, nil
	case 2:
		return &models.Resource{
			Name: split[1],
			Env:  split[0],
		}, nil
	default:
		return nil, nil
	}
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

func (h *Handler) handleGetResourceError(ea *EventAction, err error) {
	msg := msgMustSpecifyResource
	if err == e.InvalidResourceFormat {
		msg = msgResourceImproperlyFormatted
	}
	h.errorReply(ea.Event.Channel, msg)
}

func (h *Handler) errorReply(channel, msg string) {
	if msg == "" {
		msg = msgIDontKnow
	}
	h.client.PostMessage(channel, slack.MsgOptionText(msg, false))
}

func (h *Handler) reply(ea *EventAction, msg string, address bool) error {
	// If message is in DM or does not start with addressing a user, capitalize the first letter
	if !address || ea.Event.ChannelType == "im" {
		msg = fmt.Sprintf("%s%s", strings.ToUpper(msg[:1]), msg[1:])
	}

	if ea.Event.ChannelType != "im" {
		user, err := h.getUser(ea.Event.User)
		if err != nil {
			return err
		}
		if address {
			msg = fmt.Sprintf("%s %s", h.getUserDisplay(user, true), msg)
		}
	}

	_, _, err := h.client.PostMessage(ea.Event.Channel, slack.MsgOptionText(msg, false))
	return err
}

func (h *Handler) announce(ea *EventAction, user *models.User, msg string) error {
	if user != nil {
		return h.sendDM(user, msg)
	}

	_, _, err := h.client.PostMessage(ea.Event.Channel, slack.MsgOptionText(msg, false))
	return err
}

func (h *Handler) sendDM(user *models.User, msg string) error {
	_, _, c, err := h.client.OpenIMChannel(user.ID)
	if err != nil {
		return err
	}
	_, _, err = h.client.PostMessage(c, slack.MsgOptionText(msg, false))
	return err
}
