package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
)

const owner string = `@_**Maren Beam (SP2'19)**`

// this is the "id" field from zulip, and is a permanent user ID that's not secret
// Pairing Bot's owner can add their ID here for testing. ctrl+f "ownerID" to see where it's used
const ownerID = 215391

const helpMessage string = "**How to use Pairing Bot:**\n* `subscribe` to start getting matched with other Pairing Bot users for pair programming\n* `schedule monday wednesday friday` to set your weekly pairing schedule\n  * In this example, I've been set to find pairing partners for you on every Monday, Wednesday, and Friday\n  * You can schedule pairing for any combination of days in the week\n* `skip tomorrow` to skip pairing tomorrow\n  * This is valid until matches go out at 04:00 UTC\n* `unskip tomorrow` to undo skipping tomorrow\n* `status` to show your current schedule, skip status, and name\n* `count` to get the current number of subscribers\n* `unsubscribe` to stop getting matched entirely\n\nIf you've found a bug, please [submit an issue on github](https://github.com/thwidge/pairing-bot/issues)!"
const subscribeMessage string = "Yay! You're now subscribed to Pairing Bot!\nCurrently, I'm set to find pair programming partners for you on **Mondays**, **Tuesdays**, **Wednesdays**, **Thursdays**, and **Fridays**.\nYou can customize your schedule any time with `schedule` :)"
const unsubscribeMessage string = "You're unsubscribed!\nI won't find pairing partners for you unless you `subscribe`.\n\nBe well :)"
const notSubscribedMessage string = "You're not subscribed to Pairing Bot <3"
const oddOneOutMessage string = "OK this is awkward.\nThere were an odd number of people in the match-set today, which means that one person couldn't get paired. Unfortunately, it was you -- I'm really sorry :(\nI promise it's not personal, it was very much random. Hopefully this doesn't happen again too soon. Enjoy your day! <3"
const matchedMessage = "Hi you two! You've been matched for pairing :)\n\nHave fun!"
const offboardedMessage = "Hi! You've been unsubscribed from Pairing Bot.\n\nThis happens at the end of every batch, when everyone is offboarded even if they're still in batch. If you'd like to re-subscribe, just send me a message that says `subscribe`.\n\nBe well! :)"

const botEmailAddress = "pairing-bot@recurse.zulipchat.com"
const zulipAPIURL = "https://recurse.zulipchat.com/api/v1/messages"

var writeErrorMessage = fmt.Sprintf("Something went sideways while writing to the database. You should probably ping %v", owner)
var readErrorMessage = fmt.Sprintf("Something went sideways while reading from the database. You should probably ping %v", owner)

// This is a struct that gets only what
// we need from the incoming JSON payload
type incomingJSON struct {
	Data    string `json:"data"`
	Token   string `json:"token"`
	Trigger string `json:"trigger"`
	Message struct {
		SenderID         int         `json:"sender_id"`
		DisplayRecipient interface{} `json:"display_recipient"`
		SenderEmail      string      `json:"sender_email"`
		SenderFullName   string      `json:"sender_full_name"`
	} `json:"message"`
}

// Zulip has to get JSON back from the bot,
// this does that. An empty message field stops
// zulip from throwing an error at the user that
// messaged the bot, but doesn't send a response
type botResponse struct {
	Message string `json:"content"`
}

type botNoResponse struct {
	Message bool `json:"response_not_required"`
}

type parsingErr struct{ msg string }

func (e parsingErr) Error() string {
	return fmt.Sprintf("Error when parsing command: %s", e.msg)
}

// TODO this still takes a firestoreClient (but doesn't even use it?)
// TODO rename to correctnessCheck or smth
func sanityCheck(c *firestore.Client, w http.ResponseWriter, r *http.Request) (incomingJSON, error) {
	var userReq incomingJSON
	// Look at the incoming webhook and slurp up the JSON
	// Error if the JSON from Zulip istelf is bad
	err := json.NewDecoder(r.Body).Decode(&userReq)
	if err != nil {
		http.NotFound(w, r)
		return userReq, err
	}

	// validate our zulip-bot token
	// this was manually put into the database before deployment

	// TODO
	// this is probably not good, just to make this work
	adb := FirestoreAPIAuthDB{}
	adb.client = c

	ctx := context.Background()

	botAuth, err := adb.GetKey(ctx, "botauth", "token")

	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
		return userReq, err
	}
	if userReq.Token != botAuth {
		http.NotFound(w, r)
		return userReq, errors.New("unauthorized interaction attempt")
	}
	return userReq, err
}

func dispatch(rdb *FirestoreRecurserDB, cmd string, cmdArgs []string, userID string, userEmail string, userName string) (string, error) {
	var response string
	var err error

	ctx := context.Background()

	// TODO handle former readErrorMessage
	rec, err := rdb.GetByUserID(ctx, userID, userEmail, userName) // returns Recurser, error

	isSubscribed := rec.isSubscribed

	// here's the actual actions. command input from
	// the user has already been sanitized, so we can
	// trust that cmd and cmdArgs only have valid stuff in them
	switch cmd {
	case "schedule":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}
		// create a new blank schedule
		var newSchedule = map[string]interface{}{
			"monday":    false,
			"tuesday":   false,
			"wednesday": false,
			"thursday":  false,
			"friday":    false,
			"saturday":  false,
			"sunday":    false,
		}
		// populate it with the new days they want to pair on
		for _, day := range cmdArgs {
			newSchedule[day] = true
		}
		// put it in the database
		rec.schedule = newSchedule

		ctx := context.Background()

		err = rdb.Set(ctx, userID, rec)

		if err != nil {
			response = writeErrorMessage
			break
		}
		response = "Awesome, your new schedule's been set! You can check it with `status`."

	case "subscribe":
		if isSubscribed {
			response = "You're already subscribed! Use `schedule` to set your schedule."
			break
		}

		defaultSchedule := map[string]interface{}{
			"monday":    true,
			"tuesday":   true,
			"wednesday": true,
			"thursday":  true,
			"friday":    true,
			"saturday":  false,
			"sunday":    false,
		}

		newRecurser := Recurser{id: userID,
			name:               userName,
			email:              userEmail,
			isSkippingTomorrow: false,
			schedule:           defaultSchedule,
		}

		ctx := context.Background()
		err = rdb.Set(ctx, userID, newRecurser)

		if err != nil {
			response = writeErrorMessage
			break
		}
		response = subscribeMessage

	case "unsubscribe":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}

		ctx := context.Background()

		err := rdb.Delete(ctx, userID)

		if err != nil {
			response = writeErrorMessage
			break
		}
		response = unsubscribeMessage

	case "skip":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}

		rec.isSkippingTomorrow = true

		ctx := context.Background()

		err := rdb.Set(ctx, userID, rec)
		if err != nil {
			response = writeErrorMessage
			break
		}
		response = `Tomorrow: cancelled. I feel you. **I will not match you** for pairing tomorrow <3`

	case "unskip":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}
		rec.isSkippingTomorrow = false

		ctx := context.Background()

		err := rdb.Set(ctx, userID, rec)
		if err != nil {
			response = writeErrorMessage
			break
		}
		response = "Tomorrow: uncancelled! Heckin *yes*! **I will match you** for pairing tomorrow :)"

	case "status":
		if !isSubscribed {
			response = notSubscribedMessage
			break
		}
		// this particular days list is for sorting and printing the
		// schedule correctly, since it's stored in a map in all lowercase
		var daysList = []string{
			"Monday",
			"Tuesday",
			"Wednesday",
			"Thursday",
			"Friday",
			"Saturday",
			"Sunday"}

		// get their current name
		whoami := rec.name

		// get skip status and prepare to write a sentence with it
		var skipStr string
		if rec.isSkippingTomorrow {
			skipStr = " "
		} else {
			skipStr = " not "
		}

		// make a sorted list of their schedule
		var schedule []string
		for _, day := range daysList {
			// this line is a little wild, sorry. it looks so weird because we
			// have to do type assertion on both interface types
			if rec.schedule[strings.ToLower(day)].(bool) {
				schedule = append(schedule, day)
			}
		}
		// make a lil nice-lookin schedule string
		var scheduleStr string
		for i := range schedule[:len(schedule)-1] {
			scheduleStr += schedule[i] + "s, "
		}
		if len(schedule) > 1 {
			scheduleStr += "and " + schedule[len(schedule)-1] + "s"
		} else if len(schedule) == 1 {
			scheduleStr += schedule[0] + "s"
		}

		response = fmt.Sprintf("* You're %v\n* You're scheduled for pairing on **%v**\n* **You're%vset to skip** pairing tomorrow", whoami, scheduleStr, skipStr)

	case "help":
		response = helpMessage
	case "count":
		response = fmt.Sprintf("There are currently %v users subscribed to Pairing Bot.", subscriberCount(rdb))
	default:
		// this won't execute because all input has been sanitized
		// by parseCmd() and all cases are handled explicitly above
	}
	return response, err
}

func (rdb *FirestoreRecurserDB) handle(w http.ResponseWriter, r *http.Request) {
	responder := json.NewEncoder(w)

	// sanity check the incoming request
	// we only sanity check requests for handle / webhooks, i.e. user input
	userReq, err := sanityCheck(rdb.client, w, r)
	if err != nil {
		log.Println(err)
		return
	}

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

	if userReq.Trigger != "private_message" {
		err = responder.Encode(botResponse{"Hi! I'm Pairing Bot (she/her)!\n\nSend me a PM that says `subscribe` to get started :smiley:\n\n:pear::robot:\n:octopus::octopus:"})
		if err != nil {
			log.Println(err)
		}
		return
	}
	// if there aren't two 'recipients' (one sender and one receiver),
	// then don't respond. this stops pairing bot from responding in the group
	// chat she starts when she matches people
	if len(userReq.Message.DisplayRecipient.([]interface{})) != 2 {
		err = responder.Encode(botNoResponse{true})
		if err != nil {
			log.Println(err)
		}
		return
	}
	// you *should* be able to throw any freakin string at this thing and get back a valid command for dispatch()
	// if there are no commad arguments, cmdArgs will be nil
	cmd, cmdArgs, err := parseCmd(userReq.Data)
	if err != nil {
		log.Println(err)
	}
	// the tofu and potatoes right here y'all
	response, err := dispatch(rdb, cmd, cmdArgs, strconv.Itoa(userReq.Message.SenderID), userReq.Message.SenderEmail, userReq.Message.SenderFullName)
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
func (rdb *FirestoreRecurserDB) match(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	ctx := context.Background()

	// TODO handle err / empty list?
	recursersList, err := rdb.ListPairingTomorrow(ctx) // TODO handle error

	ctx = context.Background()

	skippersList, err := rdb.ListSkippingTomorrow(ctx)

	// get everyone who was set to skip today and set them back to isSkippingTomorrow = false

	ctx = context.Background()

	for _, skipper := range skippersList {
		err := rdb.UnsetSkippingTomorrow(ctx, skipper)
		if err != nil {
			log.Panic(err)
		}
	}

	// shuffle our recursers. This will not error if the list is empty
	recursersList = shuffle(recursersList)

	// if for some reason there's no matches today, we're done
	if len(recursersList) == 0 {
		log.Println("No one was signed up to pair today -- so there were no matches")
		return
	}

	// TODO
	// this is probably not good, just to make this work
	adb := FirestoreAPIAuthDB{}
	adb.client = rdb.client

	// message the peeps!
	botPassword, err := adb.GetKey(ctx, "apiauth", "key")
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
		req, err := http.NewRequest("POST", zulipAPIURL, strings.NewReader(messageRequest.Encode())) // TODO handle error
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
		req, err := http.NewRequest("POST", zulipAPIURL, strings.NewReader(messageRequest.Encode())) // TODO handle error
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

func (rdb *FirestoreRecurserDB) endofbatch(w http.ResponseWriter, r *http.Request) {
	// Check that the request is originating from within app engine
	// https://cloud.google.com/appengine/docs/flexible/go/scheduling-jobs-with-cron-yaml#validating_cron_requests
	if r.Header.Get("X-Appengine-Cron") != "true" {
		http.NotFound(w, r)
		return
	}

	// getting all the recursers
	ctx := context.Background()
	recursersList, err := rdb.GetAllUsers(ctx)

	if err != nil {
		log.Panic(err)
	}

	// message and offboard everyone (delete them from the database)

	// TODO
	// this is probably not good, just to make this work
	adb := FirestoreAPIAuthDB{}
	adb.client = rdb.client

	ctx = context.Background()

	botPassword, err := adb.GetKey(ctx, "apiauth", "key")
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

		err = rdb.Delete(ctx, recurserID)
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
		req, err := http.NewRequest("POST", zulipAPIURL, strings.NewReader(messageRequest.Encode())) // TODO handle err
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

func subscriberCount(rdb *FirestoreRecurserDB) int {

	// getting all the recursers, but only to count them

	ctx := context.Background()
	recursersList, err := rdb.GetAllUsers(ctx)

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
