package handler

import (
	"fmt"
	"strings"

	"github.com/ameliagapin/reservebot/data"
	"github.com/ameliagapin/reservebot/models"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type Handler struct {
	client *slack.Client
	data   data.Manager
}

type EventAction struct {
	Event  *slackevents.MessageEvent
	Action string
}

func New(client *slack.Client, data data.Manager) *Handler {
	return &Handler{
		client: client,
		data:   data,
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
	case "remove", "remove_dm":
		return h.remove(ea)
	case "clear", "clear_dm":
		return h.clear(ea)
	case "kick", "kick_dm":
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
		return h.reply(ea, helpText, false)
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
		user := h.getUserDisplay(q.Reservations[0].User, mention)
		msg = fmt.Sprintf("`%s` is currently reserved by %s", resource, user)
	default:
		verb := "is"
		for _, next := range q.Reservations[1:] {
			queue = append(queue, h.getUserDisplay(next.User, false))
		}
		if len(queue) > 1 {
			verb = "are"
		}
		user := h.getUserDisplay(q.Reservations[0].User, mention)
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

func (h *Handler) getResourcesFromCommaList(text string) []*models.Resource {
	ret := []*models.Resource{}
	split := strings.Split(text, ",")
	for _, s := range split {
		if len(s) > 0 {
			if r := h.parseResource(strings.Trim(s, " `")); r != nil {
				ret = append(ret, r)
			}
		}
	}
	return ret
}

func (h *Handler) parseResource(text string) *models.Resource {
	split := strings.Split(text, "|")
	switch len(split) {
	case 1:
		return &models.Resource{
			Name: split[0],
		}
	case 2:
		return &models.Resource{
			Name: split[1],
			Env:  split[0],
		}
	default:
		return nil
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

const helpText = `
Hello! I can be used via any channel that I have been added to or via DM. Regardless of where you invoke a command, there is a single reservation system that will be shared.

I can handle multiple environments or namespaces. A resource is defined as ` + "`" + `env|name` + "`" + `. If you omit the environment/namespace, the global environment will be used.

When invoking via DM, I will alert other users via DM when necessary. E.g. Releasing a resource will notify the next user that has it.

*Commands*

When invoking within a channel, you must @-mention me by adding ` + "`@reservebot`" + ` to the _beginning_ of your command.

` + "`reserve <resource>`" + ` This will reserve a given resource for the user. If the resource is currently reserved, the user will be placed into the queue. The resource should be an alphanumeric string with no spaces. A comma-separted list can be used to reserve multiple resources.

` + "`release <resource>`" + ` This will release a given resource. This command must be executed by the person who holds the resource. Upon release, the next person waiting in line will be notified that they now have the resource. The resource should be an alphanumeric string with no spaces. A comma-separted list can be used to reserve multiple resources.

` + "`status`" + ` This will provide a status of all active resources.

` + "`my status`" + ` This will provide a status of all active and queue reservations for the user.

` + "`status <resource>`" + ` This will provide a status of a given resource.

` + "`remove me from <resource>`" + ` This will remove the user from the queue for a resource.

` + "`clear <resource>`" + ` This will clear the queue for a given resource and release it.

` + "`prune`" + ` This will clear all unreserved resources from memory.

` + "`kick <@user>`" + ` This will kick the mentioned user from _all_ resources they are holding. As the user is kicked from each resource, the queue will be advanced to the next user waiting.

` + "`nuke`" + ` This will clear all reservations and all queues for all resources. This can only be done from a public channel, not a DM. There is no confirmation, so be careful.

`
