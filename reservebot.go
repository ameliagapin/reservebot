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
	"github.com/ameliagapin/reservebot/util"
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
	pruneEnabled   bool
	pruneInterval  int
	pruneExpire    int
)

func main() {
	flag.StringVar(&token, "token", util.LookupEnvOrString("SLACK_TOKEN", ""), "Slack API Token")
	flag.StringVar(&challenge, "challenge", util.LookupEnvOrString("SLACK_CHALLENGE", ""), "Slack verification token")
	flag.IntVar(&listenPort, "listen-port", util.LookupEnvOrInt("LISTEN_PORT", 666), "Listen port")
	flag.BoolVar(&debug, "debug", util.LookupEnvOrBool("DEBUG", false), "Debug mode")
	flag.StringVar(&admins, "admins", util.LookupEnvOrString("SLACK_ADMINS", ""), "Turn on administrative commands for specific admins, comma separated list")
	flag.BoolVar(&reqResourceEnv, "require-resource-env", util.LookupEnvOrBool("REQUIRE_RESOURCE_ENV", true), "Require resource reservation to include environment")
	flag.BoolVar(&pruneEnabled, "prune-enabled", util.LookupEnvOrBool("PRUNE_ENABLED", true), "Enable pruning available resources automatically")
	flag.IntVar(&pruneInterval, "prune-interval", util.LookupEnvOrInt("PRUNE_INTERVAL", 1), "Automatic pruning interval in hours")
	flag.IntVar(&pruneExpire, "prune-expire", util.LookupEnvOrInt("PRUNE_EXPIRE", 168), "Automatic prune expiration time in hours")
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

	api := slack.New(token, slack.OptionDebug(debug))

	data := data.NewMemory()

	if pruneEnabled {
		// Prune inactive resources
		log.Infof("Automatic Pruning is enabled.")
		go func() {
			for {
				time.Sleep(time.Duration(pruneInterval) * time.Hour)
				err := data.PruneInactiveResources(pruneExpire)
				if err != nil {
					log.Errorf("Error pruning resources: %+v", err)
				} else {
					log.Infof("Pruned resources")
				}
			}
		}()

	} else {
		log.Infof("Automatic pruning is disabled.")
	}

	handler := handler.New(api, data, reqResourceEnv, util.ParseAdmins(admins))

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
