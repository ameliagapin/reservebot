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
	actions["release"] = *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\srelease\s(.+)`)
	actions["clear"] = *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sclear\s(.+)`)
	actions["remove"] = *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sremove\sme\sfrom\s(.+)`)
	actions["all_status"] = *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sstatus$`)
	actions["single_status"] = *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sstatus\s([a-zA-Z0-9]+)`)
	actions["nuke"] = *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\snuke$`)
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
		case "release":
			return h.release(ev)
		case "remove":
			return h.remove(ev)
		case "clear":
			return h.clear(ev)
		case "nuke":
			return h.nuke(ev)
		case "all_status":
			return h.allStatus(ev)
		case "single_status":
			return h.singleStatus(ev)
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
		msg := fmt.Sprintf("<@%s> you must specify a resource", u.ID)
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

	if len(success) == 0 {
		return nil
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
			log.Errorf("*%s* reserved `%s`, but is currently not in the queue", u, res)
		case 1:
			msg = fmt.Sprintf("*%s* currently has `%s`", u.Name, res)
		default:
			cu, err := h.reservations.GetReservationForResource(res)
			c := ""
			if err == nil && cu != nil {
				c = fmt.Sprintf(". *%s* has it currently.", cu.User.Name)
			}
			msg = fmt.Sprintf("*%s* is %s in line for `%s`%s", u.Name, util.Ordinalize(pos), res, c)
		}
		h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
	}

	return nil
}

func (h *Handler) release(ev *slackevents.AppMentionEvent) error {
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, something went wrong inside of me.", false))
		return err
	}

	matches := h.getMatches("release", ev.Text)
	resources := h.getResourcesFromCommaList(matches[0])
	if len(resources) == 0 {
		msg := fmt.Sprintf("<@%s> you must specify a resource", u.ID)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
		return nil
	}

	success := []string{}
	for _, res := range resources {
		err := h.reservations.Remove(res, u)
		if err != nil {
			h.client.PostMessage(ev.Channel, slack.MsgOptionText(err.Error(), false))
			continue
		}
		success = append(success, res)
	}

	for _, res := range success {
		cu, err := h.reservations.GetReservationForResource(res)
		if err != nil {
			h.client.PostMessage(ev.Channel, slack.MsgOptionText(err.Error(), false))
			continue
		}
		msg := ""
		if cu == nil {
			msg = fmt.Sprintf("`%s` is now free", res)
		} else {
			msg = fmt.Sprintf("*%s* has released `%s`. <@%s>, it's all yours. Get weird.", u.Name, res, cu.User.ID)
		}

		h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
	}

	return nil
}

func (h *Handler) remove(ev *slackevents.AppMentionEvent) error {
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, something went wrong inside of me.", false))
		return err
	}

	matches := h.getMatches("remove", ev.Text)
	resources := h.getResourcesFromCommaList(matches[0])
	if len(resources) == 0 {
		msg := fmt.Sprintf("<@%s> you must specify a resource", u.ID)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
		return nil
	}

	for _, res := range resources {
		err := h.reservations.Remove(res, u)
		if err != nil {
			h.client.PostMessage(ev.Channel, slack.MsgOptionText(err.Error(), false))
			continue
		}
		cu, err := h.reservations.GetReservationForResource(res)
		if err != nil {
			h.client.PostMessage(ev.Channel, slack.MsgOptionText(err.Error(), false))
			continue
		}

		current := ". It is now free."
		if cu != nil {
			current = fmt.Sprintf(". *%s* still has it.", cu.User.Name)
		}

		msg := fmt.Sprintf("*%s* has removed themselves from the queue for `%s`%s", u.Name, res, current)

		h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
	}

	return nil
}

func (h *Handler) allStatus(ev *slackevents.AppMentionEvent) error {
	all := h.reservations.GetResources()

	if len(all) == 0 {
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("Currently, there are no reservations. The world is your oyster.", false))
		return nil
	}

	for _, r := range all {
		msg, err := h.getCurrentResText(r, false)
		if err != nil {
			log.Errorf("%+v", err)
			h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, something went wrong inside of me.", false))
			continue
		}

		h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
	}
	return nil
}

func (h *Handler) singleStatus(ev *slackevents.AppMentionEvent) error {
	resource := h.getMatches("single_status", ev.Text)

	if len(resource) == 0 {
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("You must specify a resource", false))
		return nil
	}
	msg, err := h.getCurrentResText(resource[0], false)
	if err != nil {
		log.Errorf("%+v", err)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, something went wrong inside of me.", false))
		return err
	}

	h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))

	return nil
}

func (h *Handler) clear(ev *slackevents.AppMentionEvent) error {
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, something went wrong inside of me.", false))
		return err
	}

	matches := h.getMatches("clear", ev.Text)
	resources := h.getResourcesFromCommaList(matches[0])
	if len(resources) == 0 {
		msg := fmt.Sprintf("<@%s> you must specify a resource", u.ID)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
		return nil
	}

	for _, res := range resources {
		err := h.reservations.Clear(res)
		if err != nil {
			h.client.PostMessage(ev.Channel, slack.MsgOptionText(err.Error(), false))
			continue
		}
		msg := fmt.Sprintf("`%s` has been cleared", res)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))
	}

	return nil
}

func (h *Handler) nuke(ev *slackevents.AppMentionEvent) error {
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.client.PostMessage(ev.Channel, slack.MsgOptionText("I'm sorry, something went wrong inside of me.", false))
		return err
	}

	h.reservations = models.NewReservations()

	msg := fmt.Sprintf("%s nuked the whole thing. Yikes.", h.getUserDisplay(u, true))
	h.client.PostMessage(ev.Channel, slack.MsgOptionText(msg, false))

	return nil
}

func (h *Handler) getCurrentResText(resource string, mention bool) (string, error) {
	q, err := h.reservations.GetQueueForResource(resource)
	if err != nil {
		return "", err
	}

	msg := ""
	verb := "is"
	queue := []string{}

	switch len(q) {
	case 0:
		msg = fmt.Sprintf("`%s` is free", resource)
	case 1:
		user := h.getUserDisplay(q[0].User, mention)
		msg = fmt.Sprintf("`%s` is currently reserved by %s", resource, user)
	default:
		for _, next := range q[1:] {
			queue = append(queue, "*"+next.User.Name+"*")
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
