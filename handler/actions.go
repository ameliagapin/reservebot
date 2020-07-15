package handler

import (
	"fmt"
	"regexp"

	"github.com/ameliagapin/reservebot/models"
	"github.com/ameliagapin/reservebot/util"
	log "github.com/sirupsen/logrus"
)

var (
	actions = map[string]regexp.Regexp{
		"hello":         *regexp.MustCompile(`hello.+`),
		"reserve":       *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sreserve\s(.+)`),
		"release":       *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\srelease\s(.+)`),
		"clear":         *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sclear\s(.+)`),
		"kick":          *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\skick\s\<\@([a-zA-Z0-9]+)\>`),
		"remove":        *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sremove\sme\sfrom\s(.+)`),
		"all_status":    *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sstatus$`),
		"single_status": *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\sstatus\s(.+)`),
		"my_status":     *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\smy\sstatus`),
		"nuke":          *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\snuke$`),
		"help":          *regexp.MustCompile(`(?m)^\<\@[A-Z0-9]+\>\shelp$`),

		"reserve_dm":       *regexp.MustCompile(`(?m)^reserve\s(.+)`),
		"release_dm":       *regexp.MustCompile(`(?m)^release\s(.+)`),
		"clear_dm":         *regexp.MustCompile(`(?m)^clear\s(.+)`),
		"kick_dm":          *regexp.MustCompile(`(?m)^kick\s\<\@([a-zA-Z0-9]+)\>`),
		"remove_dm":        *regexp.MustCompile(`(?m)^remove\sme\sfrom\s(.+)`),
		"all_status_dm":    *regexp.MustCompile(`(?m)^status$`),
		"single_status_dm": *regexp.MustCompile(`(?m)^status\s(.+)`),
		"my_status_dm":     *regexp.MustCompile(`(?m)^my\sstatus`),
		"nuke_dm":          *regexp.MustCompile(`(?m)^nuke$`),
		"help_dm":          *regexp.MustCompile(`(?m)^help$`),
	}
)

func (h *Handler) getAction(text string) string {
	for a, r := range actions {
		if r.MatchString(text) {
			return a
		}
	}
	return ""
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
	u, err := h.getUser(ev.User)
	if err != nil {
		log.Errorf("%+v", err)
		h.errorReply(ev.Channel, "")
		return err
	}

	userOnly := false
	switch ea.Action {
	case "my_status", "my_status_dm":
		userOnly = true
	}

	all := h.reservations.GetResources()

	if len(all) == 0 {
		return h.reply(ea, "Currently, there are no reservations. The world is your oyster.", false)
	}

	resp := ""
	for _, r := range all {
		if userOnly {
			// Discarding the err here. Func returns 0 when there's an err so we'll use that as an indication
			// to just skip
			pos, _ := h.reservations.GetPosition(r, u)
			if pos <= 0 {
				continue
			}
		}
		msg, err := h.getCurrentResText(r, false)
		if err != nil {
			log.Errorf("%+v", err)
			h.errorReply(ev.Channel, "")
			continue
		}

		resp += msg + "\n"
	}

	if resp == "" {
		resp = "Currently, there are no reservations. The world is your oyster."
	}
	// Only address the user if they asked for *their* status
	h.reply(ea, resp, userOnly)

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
