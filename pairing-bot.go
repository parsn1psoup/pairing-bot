package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const owner string = `@_**Maren Beam (SP2'19)**`

// this is the "id" field from zulip, and is a permanent user ID that's not secret
// Pairing Bot's owner can add their ID here for testing. ctrl+f "ownerID" to see where it's used
const ownerID = 215391

type parsingErr struct{ msg string }

func (e parsingErr) Error() string {
	return fmt.Sprintf("Error when parsing command: %s", e.msg)
}

type PairingLogic struct {
	rdb RecurserDB
	adb APIAuthDB
	sc  someClient
}

func (pl *PairingLogic) handle(w http.ResponseWriter, r *http.Request) {
	responder := json.NewEncoder(w)

	// check and authorize the incoming request
	// observation: we only validate requests for /webhooks, i.e. user input through zulip

	ctx := context.Background()
	err := pl.sc.validateJSON(ctx, r)
	if err != nil {
		http.NotFound(w, r)
	}

	botAuth, err := pl.adb.GetKey(ctx, "botauth", "token")
	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
	}

	if !pl.sc.validateAuthCreds(ctx, botAuth) {
		http.NotFound(w, r)
	}

	// TODO handle this with the new data types.
	// how to toggle maintenance mode witout commenting out code? run with env variable in app.yaml? https://cloud.google.com/appengine/docs/standard/python/config/appref
	/*
		// for testing only
		// this responds with a maintenance message and quits if the request is coming from anyone other than the owner
		if userReq.Message.SenderID != ownerID {
			err = responder.Encode(botResponse{`pairing bot is down for maintenance`})
			if err != nil {
				log.Println(err)
			}
			return
		}
	*/

	intro := pl.sc.validateInteractionType(ctx)
	if intro != nil {
		err = responder.Encode(intro)
		if err != nil {
			log.Println(err)
		}
		return
	}

	ignore := pl.sc.ignoreInteractionType(ctx)
	if ignore != nil {
		err = responder.Encode(ignore)
		if err != nil {
			log.Println(err)
		}
		return
	}
	// you *should* be able to throw any string at this thing and get back a valid command for dispatch()
	// if there are no commad arguments, cmdArgs will be nil
	cmd, cmdArgs, err := pl.sc.sanitizeUserInput(ctx)
	if err != nil {
		log.Println(err)
	}

	// the tofu and potatoes right here y'all
	userData := pl.sc.extractUserData(ctx)

	response, err := dispatch(pl, cmd, cmdArgs, userData.userID, userData.userEmail, userData.userName)
	if err != nil {
		log.Println(err)
	}

	err = responder.Encode(botResponse{response})
	if err != nil {
		log.Println(err)
	}
}

func nope(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}

// "match" makes matches for pairing, and messages those people to notify them of their match
// it runs once per day at 8am (it's triggered with app engine's cron service)
func (pl *PairingLogic) match(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	ctx := context.Background()

	recursersList, err := pl.rdb.ListPairingTomorrow(ctx)
	if err != nil {
		log.Printf("Could not get list of recursers from DB: %s\n", err)
	}
	// TODO do we want to return in case of errors here?

	ctx = context.Background()

	skippersList, err := pl.rdb.ListSkippingTomorrow(ctx)
	if err != nil {
		log.Printf("Could not get list of skippers from DB: %s\n", err)
	}

	// get everyone who was set to skip today and set them back to isSkippingTomorrow = false

	ctx = context.Background()

	for _, skipper := range skippersList {
		err := pl.rdb.UnsetSkippingTomorrow(ctx, skipper)
		if err != nil {
			log.Printf("Could not unset skipping for recurser %v: %s\n", skipper.id, err)
		}
	}

	// shuffle our recursers. This will not error if the list is empty
	recursersList = shuffle(recursersList)

	// if for some reason there's no matches today, we're done
	if len(recursersList) == 0 {
		log.Println("No one was signed up to pair today -- so there were no matches")
		return
	}

	// message the peeps!
	botPassword, err := pl.adb.GetKey(ctx, "apiauth", "key")
	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
	}
	botUsername := botEmailAddress
	zulipClient := &http.Client{}

	// if there's an odd number today, message the last person in the list
	// and tell them they don't get a match today, then knock them off the list
	if len(recursersList)%2 != 0 {
		recurser := recursersList[len(recursersList)-1]
		recursersList = recursersList[:len(recursersList)-1]
		log.Println("Someone was the odd-one-out today")
		messageRequest := url.Values{}
		messageRequest.Add("type", "private")
		messageRequest.Add("to", recurser.email)
		messageRequest.Add("content", oddOneOutMessage)
		req, err := http.NewRequest("POST", zulipAPIURL, strings.NewReader(messageRequest.Encode()))
		if err != nil {
			log.Printf("Error when trying to send oddOneOutMessage: %s\n", err)
		}
		req.SetBasicAuth(botUsername, botPassword)
		req.Header.Set("content-type", "application/x-www-form-urlencoded")
		resp, err := zulipClient.Do(req)
		if err != nil {
			log.Panic(err)
		}
		defer resp.Body.Close()
		respBodyText, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
		}
		log.Println(string(respBodyText))
	}

	for i := 0; i < len(recursersList); i += 2 {
		messageRequest := url.Values{}
		messageRequest.Add("type", "private")
		messageRequest.Add("to", recursersList[i].email+", "+recursersList[i+1].email)
		messageRequest.Add("content", matchedMessage)
		req, err := http.NewRequest("POST", zulipAPIURL, strings.NewReader(messageRequest.Encode()))
		if err != nil {
			log.Printf("Error when trying to send matchedMessage: %s\n", err)
		}
		req.SetBasicAuth(botUsername, botPassword)
		req.Header.Set("content-type", "application/x-www-form-urlencoded")
		resp, err := zulipClient.Do(req)
		if err != nil {
			log.Panic(err)
		}
		defer resp.Body.Close()
		respBodyText, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
		}
		log.Println(string(respBodyText))
		log.Println(recursersList[i].email, "was", "matched", "with", recursersList[i+1].email)
	}
}

func (pl *PairingLogic) endofbatch(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	// getting all the recursers
	ctx := context.Background()
	recursersList, err := pl.rdb.GetAllUsers(ctx)

	if err != nil {
		log.Panic(err)
	}

	// message and offboard everyone (delete them from the database)

	ctx = context.Background()

	botPassword, err := pl.adb.GetKey(ctx, "apiauth", "key")
	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
	}
	botUsername := botEmailAddress

	zulipClient := &http.Client{}

	ctx = context.Background() // TODO my use of contexts is definitely wrong

	for i := 0; i < len(recursersList); i++ {
		recurserID := recursersList[i].id
		recurserEmail := recursersList[i].email
		messageRequest := url.Values{}
		var message string

		err = pl.rdb.Delete(ctx, recurserID)
		if err != nil {
			log.Println(err)
			message = fmt.Sprintf("Uh oh, I was trying to offboard you since it's the end of batch, but something went wrong. Consider messaging %v to let them know this happened.", owner)
		} else {
			log.Println("A user was offboarded because it's the end of a batch.")
			message = offboardedMessage
		}

		messageRequest.Add("type", "private")
		messageRequest.Add("to", recurserEmail)
		messageRequest.Add("content", message)
		req, err := http.NewRequest("POST", zulipAPIURL, strings.NewReader(messageRequest.Encode()))
		if err != nil {
			log.Printf("Error when trying to send offboarding message to %s: %s\n", recurserID, err)
		}
		req.SetBasicAuth(botUsername, botPassword)
		req.Header.Set("content-type", "application/x-www-form-urlencoded")
		resp, err := zulipClient.Do(req)
		if err != nil {
			log.Panic(err)
		}
		defer resp.Body.Close()
		respBodyText, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Println(err)
		}
		log.Println(string(respBodyText))
	}
}

func subscriberCount(pl *PairingLogic) int {

	// getting all the recursers, but only to count them

	ctx := context.Background()
	recursersList, err := pl.rdb.GetAllUsers(ctx)

	if err != nil {
		log.Panic(err)
	}

	return len(recursersList)
}

// this shuffles our recursers.
func shuffle(slice []Recurser) []Recurser {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ret := make([]Recurser, len(slice))
	perm := r.Perm(len(slice))
	for i, randIndex := range perm {
		ret[i] = slice[randIndex]
	}
	return ret
}
