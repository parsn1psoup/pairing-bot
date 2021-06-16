package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
)

const botEmailAddress = "pairing-bot@recurse.zulipchat.com"
const zulipAPIURL = "https://recurse.zulipchat.com/api/v1/messages"

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

type UserDataFromJSON struct {
	userID    string
	userEmail string
	userName  string
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

type someClient interface {
	validateJSON(ctx context.Context, r *http.Request) error
	validateAuthCreds(ctx context.Context, tokenFromDB string) bool
	validateInteractionType(ctx context.Context) *botResponse
	ignoreInteractionType(ctx context.Context) *botNoResponse
	sanitizeUserInput(ctx context.Context) (string, []string, error)
	extractUserData(ctx context.Context) *UserDataFromJSON // does this need an error return value? anything that hasn't been validated previously?
}

type zulipClient struct {
	json incomingJSON
}

func (zc *zulipClient) validateJSON(ctx context.Context, r *http.Request) error {
	var userReq incomingJSON
	// Look at the incoming webhook and slurp up the JSON
	// Error if the JSON from Zulip itself is bad
	err := json.NewDecoder(r.Body).Decode(&userReq)

	if err == nil {
		zc.json = userReq
	}
	return err
}

func (zc *zulipClient) validateAuthCreds(ctx context.Context, tokenFromDB string) bool {
	if zc.json.Token != tokenFromDB {
		log.Println("Unauthorized interaction attempt")
		return false
	}
	return true
}

// if the zulip msg is posted in a stream, don't treat it as a command
func (zc *zulipClient) validateInteractionType(ctx context.Context) *botResponse {
	if zc.json.Trigger != "private_message" {
		return &botResponse{"Hi! I'm Pairing Bot (she/her)!\n\nSend me a PM that says `subscribe` to get started :smiley:\n\n:pear::robot:\n:octopus::octopus:"}
	}
	return nil
}

// if there aren't two 'recipients' (one sender and one receiver),
// then don't respond. this stops pairing bot from responding in the group
// chat she starts when she matches people
func (zc *zulipClient) ignoreInteractionType(ctx context.Context) *botNoResponse {
	if len(zc.json.Message.DisplayRecipient.([]interface{})) != 2 {
		return &botNoResponse{true}
	}
	return nil
}

func (zc *zulipClient) sanitizeUserInput(ctx context.Context) (string, []string, error) {
	return parseCmd(zc.json.Data)
}

func (zc *zulipClient) extractUserData(ctx context.Context) *UserDataFromJSON {
	return &UserDataFromJSON{
		userID:    strconv.Itoa(zc.json.Message.SenderID),
		userEmail: zc.json.Message.SenderEmail,
		userName:  zc.json.Message.SenderFullName,
	}
}

// TODO add mocks
