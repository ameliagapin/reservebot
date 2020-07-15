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
	actions = map[string]regexp.Regexp{
		"hello":            *regexp.MustCompile(`hello.+`),
		"reserve":          *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sreserve\s(.+)`),
		"release":          *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\srelease\s(.+)`),
		"clear":            *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sclear\s(.+)`),
		"kick":             *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\skick\s\<\@([a-zA-Z0-9]+)\>`),
		"remove":           *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sremove\sme\sfrom\s(.+)`),
		"all_status":       *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sstatus$`),
		"single_status":    *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sstatus\s(.+)`),
		"nuke":             *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\snuke$`),
		"help":             *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\shelp$`),
		"reserve_dm":       *regexp.MustCompile(`(?m)^reserve\s(.+)`),
		"release_dm":       *regexp.MustCompile(`(?m)^release\s(.+)`),
		"clear_dm":         *regexp.MustCompile(`(?m)^clear\s(.+)`),
		"kick_dm":          *regexp.MustCompile(`(?m)^kick\s\<\@([a-zA-Z0-9]+)\>`),
		"remove_dm":        *regexp.MustCompile(`(?m)^remove\sme\sfrom\s(.+)`),
		"all_status_dm":    *regexp.MustCompile(`(?m)^status$`),
		"single_status_dm": *regexp.MustCompile(`(?m)^status\s(.+)`),
		"nuke_dm":          *regexp.MustCompile(`(?m)^nuke$`),
		"help_dm":          *regexp.MustCompile(`(?m)^help$`),
		// This regex was an attempt to pull all resources in comma separated list in repeating capture groups. It did not work
		//"reserve" : *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sreserve\s([a-zA-Z0-9]+)(?:\,\s([a-zA-Z0-9]+))?`),
	}
)

type Handler struct {
	client       *slack.Client
	reservations *models.Reservations
}

type EventAction struct {
	Event  *slackevents.MessageEvent
	Action string
}

func New(client *slack.Client) *Handler {
	return &Handler{
		client:       client,
		reservations: models.NewReservations(),
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
	case "all_status", "all_status_dm":
		return h.allStatus(ea)
	case "single_status", "single_status_dm":
		return h.singleStatus(ea)
	case "help", "help_dm":
		return h.reply(ea, helpText, false)
	default:
		return h.reply(ea, "I'm sorry, I don't know what to do with that request", false)
	}

	return nil
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

func (h *Handler) reserve(ea *EventAction) error {
	ev := ea.Event
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.errorReply(ev.Channel, "")
		return err
	}

	matches := h.getMatches(ea.Action, ev.Text)
	resources := h.getResourcesFromCommaList(matches[0])
	if len(resources) == 0 {
		return h.reply(ea, "you must specify a resource", true)
	}

	success := []string{}
	for _, res := range resources {
		err := h.reservations.Add(res, u)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}
		success = append(success, res)
	}

	if len(success) == 0 {
		return nil
	}

	for _, res := range success {
		pos, err := h.reservations.GetPosition(res, u)
		if err != nil {
			// This case really should never happen here, as we are only looping through our success cases
			log.Errorf("%+v", err)
			h.errorReply(ev.Channel, "")
			return err
		}
		switch pos {
		case 0:
			log.Errorf("%s reserved `%s`, but is currently not in the queue", h.getUserDisplay(u, false), res)
		case 1:
			msg := fmt.Sprintf("you currently have `%s`", res)
			if ev.ChannelType != "im" {
				msg = fmt.Sprintf("%s currently has `%s`", h.getUserDisplay(u, true), res)
			}
			err = h.reply(ea, msg, false)
			if err != nil {
				log.Errorf("%+v", err)
			}
		default:
			cu, err := h.reservations.GetReservationForResource(res)
			c := ""
			if err == nil && cu != nil {
				c = fmt.Sprintf(". %s has it currently.", h.getUserDisplay(cu.User, false))
			}
			msg := fmt.Sprintf("you are %s in line for `%s`%s", util.Ordinalize(pos), res, c)
			err = h.reply(ea, msg, true)
			if err != nil {
				log.Errorf("%+v", err)
			}
		}
	}

	return nil
}

func (h *Handler) release(ea *EventAction) error {
	ev := ea.Event
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.errorReply(ev.Channel, "")
		return err
	}

	matches := h.getMatches(ea.Action, ev.Text)
	resources := h.getResourcesFromCommaList(matches[0])
	if len(resources) == 0 {
		h.reply(ea, "you must specify a resource", true)
		return nil
	}

	success := []string{}
	for _, res := range resources {
		pos, err := h.reservations.GetPosition(res, u)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}
		if pos != 1 {
			h.reply(ea, "you cannot release a resource that you do not currently have. Please use `remove` instead.", true)
			continue
		}

		err = h.reservations.Remove(res, u)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}
		success = append(success, res)
	}

	for _, res := range success {
		cu, err := h.reservations.GetReservationForResource(res)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}

		if ea.Event.ChannelType == "im" {
			// Confirm for user
			msg := fmt.Sprintf("You have released `%s`", res)
			h.reply(ea, msg, false)

			if cu != nil {
				// Let next user know they are up
				msg = fmt.Sprintf("%s has released %s. It's all yours. Get weird.", h.getUserDisplay(u, false), res)
				h.announce(ea, cu.User, msg)
			}
		} else {
			msg := "It is now free."
			if cu != nil {
				msg = fmt.Sprintf("%s it's all yours. Get weird.", h.getUserDisplay(cu.User, true))
			}
			msg = fmt.Sprintf("%s has released `%s`. %s", h.getUserDisplay(u, false), res, msg)
			h.reply(ea, msg, false)
		}
	}

	return nil
}

func (h *Handler) remove(ea *EventAction) error {
	ev := ea.Event
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.errorReply(ev.Channel, "")
		return err
	}

	matches := h.getMatches(ea.Action, ev.Text)
	resources := h.getResourcesFromCommaList(matches[0])
	if len(resources) == 0 {
		h.errorReply(ev.Channel, "you must specify a resource")
		return nil
	}

	for _, res := range resources {
		pos, err := h.reservations.GetPosition(res, u)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}
		if pos == 0 {
			h.reply(ea, "you cannot remove yourself from a resource that you currently have.", true)
			continue
		} else if pos == 1 {
			h.reply(ea, "you cannot remove yourself from a resource that you currently have. Please use `release` instead.", true)
			continue
		}

		err = h.reservations.Remove(res, u)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}
		cu, err := h.reservations.GetReservationForResource(res)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}

		if ev.ChannelType == "im" {
			// We will need to confirm to the user
			h.reply(ea, fmt.Sprintf("you have removed yourself from `%s`", res), false)
		} else {
			// We only need to send one message in channel
			current := ". It is free."
			if cu != nil {
				current = fmt.Sprintf(". %s still has it.", h.getUserDisplay(cu.User, false))
			}
			msg := fmt.Sprintf("%s has removed themselves from the queue for `%s`%s", h.getUserDisplay(u, true), res, current)
			h.reply(ea, msg, false)
		}
	}

	return nil
}

func (h *Handler) allStatus(ea *EventAction) error {
	ev := ea.Event
	all := h.reservations.GetResources()

	if len(all) == 0 {
		return h.reply(ea, "currently, there are no reservations. The world is your oyster.", false)
	}

	for _, r := range all {
		msg, err := h.getCurrentResText(r, false)
		if err != nil {
			log.Errorf("%+v", err)
			h.errorReply(ev.Channel, "")
			continue
		}

		h.reply(ea, msg, false)
	}
	return nil
}

func (h *Handler) singleStatus(ea *EventAction) error {
	ev := ea.Event
	resource := h.getMatches(ea.Action, ev.Text)

	if len(resource) == 0 {
		h.errorReply(ev.Channel, "you must specify a resource")
		return nil
	}
	msg, err := h.getCurrentResText(resource[0], false)
	if err != nil {
		log.Errorf("%+v", err)
		h.errorReply(ev.Channel, "")
		return err
	}

	h.reply(ea, msg, false)

	return nil
}

func (h *Handler) clear(ea *EventAction) error {
	ev := ea.Event
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.errorReply(ev.Channel, "")
		return err
	}

	matches := h.getMatches(ea.Action, ev.Text)
	resources := h.getResourcesFromCommaList(matches[0])
	if len(resources) == 0 {
		h.errorReply(ev.Channel, "you must specify a resource")
		return nil
	}

	for _, res := range resources {
		allRes, err := h.reservations.GetQueueForResource(res)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}

		err = h.reservations.Clear(res)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}

		msg := fmt.Sprintf("`%s` has been cleared", res)
		h.reply(ea, msg, false)

		// If request was via IM, we need to notify other users
		if ev.ChannelType == "im" {
			for _, r := range allRes {
				if r.User.ID != ev.User {
					h.announce(ea, r.User, fmt.Sprintf("%s cleared `%s`", h.getUserDisplay(u, true), res))
				}
			}
		}
	}

	return nil
}

func (h *Handler) kick(ea *EventAction) error {
	ev := ea.Event
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.errorReply(ev.Channel, "")
		return err
	}

	matches := h.getMatches(ea.Action, ev.Text)
	if len(matches) != 1 {
		h.reply(ea, "you must specify a user to kick", true)
		return nil
	}
	uToKick, err := h.getUser(matches[0])
	if err != nil {
		log.Errorf("%+v", err)
		h.reply(ea, "I'm sorry, I don't know who that is.", true)
		return err
	}

	count := 0
	for _, res := range h.reservations.GetResources() {
		pos, err := h.reservations.GetPosition(res, uToKick)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}
		if pos != 1 {
			continue
		}

		err = h.reservations.Remove(res, uToKick)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}
		count++

		cu, err := h.reservations.GetReservationForResource(res)
		if err != nil {
			h.errorReply(ev.Channel, err.Error())
			continue
		}

		if ev.ChannelType == "im" {
			// We will need to confirm to the user
			h.reply(ea, fmt.Sprintf("you have removed %s from `%s`", h.getUserDisplay(uToKick, true), res), false)

			// If someone now has the resource, we must alert them
			if cu != nil {
				msg := fmt.Sprintf("%s has been kicked from `%s`. It's all yours. Get weird.", h.getUserDisplay(uToKick, false), res)
				h.announce(ea, cu.User, msg)
			}

			// Alert user who was kicked
			msg := fmt.Sprintf("%s kicked you from `%s`", h.getUserDisplay(u, true), res)
			h.announce(ea, uToKick, msg)
		} else {
			// We only need to send one message in channel
			current := ". It is now free."
			if cu != nil {
				current = fmt.Sprintf(". %s has it currently.", h.getUserDisplay(cu.User, false))
			}

			msg := fmt.Sprintf("%s has been removed from the queue for `%s`%s", h.getUserDisplay(u, false), res, current)
			h.reply(ea, msg, false)
		}
	}

	msg := fmt.Sprintf("%s has been kicked from %d resouce(s)", h.getUserDisplay(uToKick, true), count)
	h.reply(ea, msg, false)

	// User will need to be alerted
	return nil
}

func (h *Handler) nuke(ea *EventAction) error {
	ev := ea.Event
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.errorReply(ev.Channel, "")
		return err
	}

	h.reservations = models.NewReservations()

	msg := fmt.Sprintf("%s nuked the whole thing. Yikes.", h.getUserDisplay(u, true))
	h.reply(ea, msg, false)

	return nil
}

func (h *Handler) getCurrentResText(resource string, mention bool) (string, error) {
	q, err := h.reservations.GetQueueForResource(resource)
	if err != nil {
		return "", err
	}

	msg := ""
	queue := []string{}

	switch len(q) {
	case 0:
		msg = fmt.Sprintf("`%s` is free", resource)
	case 1:
		user := h.getUserDisplay(q[0].User, mention)
		msg = fmt.Sprintf("`%s` is currently reserved by %s", resource, user)
	default:
		verb := "is"
		for _, next := range q[1:] {
			queue = append(queue, h.getUserDisplay(next.User, false))
		}
		if len(queue) > 1 {
			verb = "are"
		}
		user := h.getUserDisplay(q[0].User, mention)
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
			ret = append(ret, strings.Trim(s, " `"))
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

func (h *Handler) errorReply(channel, msg string) {
	if msg == "" {
		msg = "I'm sorry, something went wrong inside of me"
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

When invoking via DM, I will alert other users via DM when necessary. E.g. Releasing a resource will notify the next user that has it.

*Commands*

When invoking within a channel, you must @-mention me by adding ` + "`@reservebot`" + ` to the _beginning_ of your command.

` + "`reserve <resource>`" + ` This will reserve a given resource for the user. If the resource is currently reserved, the user will be placed into the queue. The resource should be an alphanumeric string with no spaces. A comma-separted list can be used to reserve multiple resources.

` + "`release <resource>`" + ` This will release a given resource. This command must be executed by the person who holds the resource. Upon release, the next person waiting in line will be notified that they now have the resource. The resource should be an alphanumeric string with no spaces. A comma-separted list can be used to reserve multiple resources.

` + "`status`" + ` This will provide a status of all active resources.

` + "`status <resource>`" + ` This will provide a status of a given resource.

` + "`remove me from <resource>`" + ` This will remove the user from the queue for a resource.

` + "`clear <resource>`" + ` This will clear the queue for a given resource and release it.

` + "`kick <@user>`" + ` This will kick the mentioned user from _all_ resources they are holding. As the user is kicked from each resource, the queue will be advanced to the next user waiting.

` + "`nuke`" + ` This will clear all reservations and all queues for all resources. This can only be done from a public channel, not a DM. There is no confirmation, so be careful.

`
