package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/ameliagapin/reservebot/data"
	"github.com/ameliagapin/reservebot/handler"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

var (
	token          string
	challenge      string
	listenPort     int
	debug          bool
	reqResourceEnv bool
)

func main() {
	flag.StringVar(&token, "token", "", "Slack API Token")
	flag.StringVar(&challenge, "challenge", "", "Slack verification token")
	flag.IntVar(&listenPort, "listen-port", 666, "Listen port")
	flag.BoolVar(&debug, "debug", false, "Debug mode")
	flag.BoolVar(&reqResourceEnv, "require-resource-env", true, "Require resource reservation to include environment")
	flag.Parse()

	if token == "" {
		log.Error("Slack token is required")
		return
	}
	if challenge == "" {
		log.Error("Slack verification token is required")
		return
	}

	api := slack.New(token, slack.OptionDebug(debug))

	data := data.NewMemory()
	// Prune inactive resources once an hour
	go func() {
		for {
			time.Sleep(time.Hour)
			err := data.PruneInactiveResources(168) // one week
			if err != nil {
				log.Errorf("Error pruning resources: %+v", err)
			} else {
				log.Infof("Pruned resources")
			}
		}
	}()

	handler := handler.New(api, data, reqResourceEnv)

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		body := buf.String()

		api.Debugf("Request: %s", body)

		eventsAPIEvent, err := slackevents.ParseEvent(
			json.RawMessage(body),
			slackevents.OptionVerifyToken(
				&slackevents.TokenComparator{VerificationToken: challenge},
			),
		)

		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Errorf("%+v", err)
			return
		}

		switch eventsAPIEvent.Type {
		case slackevents.URLVerification:
			var r *slackevents.ChallengeResponse
			err := json.Unmarshal([]byte(body), &r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
			}
			w.Header().Set("Content-Type", "text")
			w.Write([]byte(r.Challenge))
		case slackevents.CallbackEvent:
			err := handler.CallbackEvent(eventsAPIEvent)
			if err != nil {
				log.Errorf("%+v", err)
			}
		default:
		}
	})

	log.Infof("Server listening on port %d", listenPort)

	http.ListenAndServe(fmt.Sprintf(":%v", listenPort), nil)

}
