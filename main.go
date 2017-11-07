/*
Copyright 2017 by the contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	slack "github.com/ashwanthkumar/slack-go-webhook"
	"github.com/gorilla/websocket"
	"github.com/jmoiron/jsonq"
	"github.com/sirupsen/logrus"
)

var log = logrus.New()
var certStreamURL = "wss://certstream.calidog.io"

func main() {
	// get the Slack webhook URL
	webhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	if webhookURL == "" {
		log.Fatal("SLACK_WEBHOOK_URL must be set")
	}

	// get and compile the domain pattern regex
	domainPattern := os.Getenv("DOMAIN_PATTERN")
	if domainPattern == "" {
		log.Fatal("DOMAIN_PATTERN must be set")
	}
	domainRegex, err := regexp.Compile(domainPattern)
	if err != nil {
		log.WithError(err).Fatal("invalid DOMAIN_PATTERN")
	}

	// connect to certstream via secure websocket
	conn, _, err := websocket.DefaultDialer.Dial(certStreamURL, nil)
	if err != nil {
		log.WithError(err).Fatal("could not connect to certstream")
	}
	defer conn.Close()

	// loop over each message sent in the websocket
	log.WithField("domainPattern", domainRegex.String()).Info("watching for certificates")
	for {
		// read a JSON message from the websocket and parse it using jsonq
		var msg interface{}
		err = conn.ReadJSON(&msg)
		if err != nil {
			log.WithError(err).Fatalf("error decoding JSON")
		}
		jq := jsonq.NewQuery(msg)

		// skip everything that's not a "certificate_update" (e.g., heartbeats)
		if t, _ := jq.String("message_type"); t != "certificate_update" {
			continue
		}

		// pull the list of all the domains named in the leaf certificate (CN and SANs)
		domains, err := jq.ArrayOfStrings("data", "leaf_cert", "all_domains")
		if err != nil {
			log.WithError(err).Error("couldn't get domains")
			continue
		}

		// if none of the domains match our regex, we're done
		match := false
		for _, domain := range domains {
			match = match || domainRegex.MatchString(domain)
		}
		if !match {
			continue
		}

		// otherwise pull the certificate fingerprint
		fingerprint, err := jq.String("data", "leaf_cert", "fingerprint")
		if err != nil {
			log.WithError(err).Error("could not parse fingerprint from matching certificate")
		}

		// wrap each domain in backticks for a prettier Slack message
		formattedDomains := []string{}
		for _, domain := range domains {
			formattedDomains = append(formattedDomains, "`"+domain+"`")
		}

		// post the Slack message
		payload := slack.Payload{
			Text: fmt.Sprintf(
				"Found matching certificate for %s: https://crt.sh/?q=%s",
				strings.Join(formattedDomains, ","),
				strings.Replace(fingerprint, ":", "", -1),
			),
		}
		for _, err := range slack.Send(webhookURL, "", payload) {
			log.WithError(err).WithField("fingerprint", fingerprint).Error("error sending webhook")
		}
	}
}
