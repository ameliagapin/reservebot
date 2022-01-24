package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
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
	admins         string
	reqResourceEnv bool
)

func LookupEnvOrString(key string, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}

func LookupEnvOrInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok {
		v, err := strconv.Atoi(val)
		if err != nil {
			log.Fatalf("LookupEnvOrInt[%s]: %v", key, err)
		}
		return v
	}
	return defaultVal
}

func LookupEnvOrBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if val == "true" {
			return true
		} else {
			return false
		}
	}
	return defaultVal
}

func main() {
	flag.StringVar(&token, "token", LookupEnvOrString("SLACK_TOKEN", ""), "Slack API Token")
	flag.StringVar(&challenge, "challenge", LookupEnvOrString("SLACK_CHALLENGE", ""), "Slack verification token")
	flag.IntVar(&listenPort, "listen-port", LookupEnvOrInt("LISTEN_PORT", 666), "Listen port")
	flag.BoolVar(&debug, "debug", LookupEnvOrBool("DEBUG", false), "Debug mode")
	flag.StringVar(&admins, "admins", LookupEnvOrString("SLACK_ADMINS", ""), "Turn on administrative commands for specific admins, comma separated list")
	flag.BoolVar(&reqResourceEnv, "require-resource-env", LookupEnvOrBool("REQUIRE_RESOURCE_ENV", true), "Require resource reservation to include environment")
	flag.Parse()

	// Make sure required vars are set
	if token == "" {
		log.Error("Slack token is required")
		return
	}
	if challenge == "" {
		log.Error("Slack verification token is required")
		return
	}

	// Convert admins list into slice
	var admins_ary []string
	if len(admins) > 0 {
		if strings.Contains(admins, ",") {
			admins_ary = strings.Split(admins, ",")
		} else {
			admins_ary = append(admins_ary, admins)
		}
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

	handler := handler.New(api, data, reqResourceEnv, admins_ary)

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
