package main

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
)

// this is what we send to / receive from Firestore
// var recurser = map[string]interface{}{
// 	"id":                 "string",
// 	"name":               "string",
// 	"email":              "string",
// 	"isSkippingTomorrow": false,
// 	"schedule": map[string]interface{}{
// 		"monday":    false,
// 		"tuesday":   false,
// 		"wednesday": false,
// 		"thursday":  false,
// 		"friday":    false,
// 		"saturday":  false,
// 		"sunday":    false,
// 	},
// }

type Recurser struct {
	id                 string
	name               string
	email              string
	isSkippingTomorrow bool
	schedule           map[string]interface{}
	isSubscribed       bool
}

// Subscriber List Lookup

type RecurserDB interface {
	GetByUserID(ctx context.Context, userID string) (Recurser, error)
	Set(ctx context.Context, userID string, recurser Recurser) error
	Delete(ctx context.Context, userID string) error
	ListPairingTomorrow(ctx context.Context) ([]Recurser, error)
	ListSkippingTomorrow(ctx context.Context) ([]Recurser, error)
	SetSkippingTomorrow(ctx context.Context, userID string) error
	UnsetSkippingTomorrow(ctx context.Context, userID string) error
}

type FirestoreRecurserDB struct {
	client *firestore.Client
}

func (f *FirestoreRecurserDB) GetByUserID(ctx context.Context, userID, userEmail, userName string) (Recurser, error) {
	// get the users "document" (database entry) out of firestore
	// we temporarily keep it in 'doc'
	doc, err := f.client.Collection("recursers").Doc(userID).Get(ctx)
	// this says "if there's an error, and if that error was not document-not-found"
	if err != nil && grpc.Code(err) != codes.NotFound {
		return Recurser{}, err
	}

	// if there's a db entry, that means they were already subscribed to pairing bot
	// if there's not, they were not subscribed
	isSubscribed := doc.Exists()

	// if the user is in the database, get their current state into this map
	// also assign their zulip name to the name field, just in case it changed
	// also assign their email, for the same reason
	var recurser map[string]interface{}

	if isSubscribed {
		recurser = doc.Data()
		recurser["name"] = userName
		recurser["email"] = userEmail
	}

	// now put the data from the recurser map into a Recurser struct
	r := MapToStruct(recurser)
	r.isSubscribed = isSubscribed
	return r, nil
}

func MapToStruct(m map[string]interface{}) Recurser {
	// isSubscribed is missing here because it's not in the map
	return Recurser{id: m["id"],
		name:               m["name"],
		email:              m["email"],
		isSkippingTomorrow: m["isSkippingTomorrow"],
		schedule:           m["schedule"],
	}
}

func (f *FirestoreRecurserDB) Set(ctx context.Context, userID string, recurser Recurser) error {

	r := recurser.ConvertToMap()
	_, err = f.client.Collection("recursers").Doc(userID).Set(ctx, r, firestore.MergeAll)
	return err

}

func (f *FirestoreRecurserDB) Delete(ctx context.Context, userID string) error {
	_, err = f.client.Collection("recursers").Doc(userID).Delete(ctx)
	return err
}

func (f *FirestoreRecurserDB) ListPairingTomorrow(ctx context.Context) ([]Recurser, error) {
	return nil, nil
}

func (f *FirestoreRecurserDB) ListSkippingTomorrow(ctx context.Context) ([]Recurser, error) {
	return nil, nil
}

func (f *FirestoreRecurserDB) SetSkippingTomorrow(ctx context.Context, userID string) error {
	return nil
}

func (f *FirestoreRecurserDB) UnsetSkippingTomorrow(ctx context.Context, userID string) error {
	return nil
}

func (r *Recurser) ConvertToMap() map[string]interface{} {
	return map[string]interface{}{
		"id":                 r.id,
		"name":               r.name,
		"email":              r.email,
		"isSkippingTomorrow": r.isSkippingTomorrow,
		"schedule":           r.schedule,
	}
}

type MockRecurserDB struct{}

func (m *MockRecurserDB) GetByUserID(ctx context.Context, userID string) (Recurser, error) {
	return Recurser{}, nil
}

func (m *MockRecurserDB) Set(ctx context.Context, userID string, recurser Recurser) error {
	return Recurser{}, nil
}

func (m *MockRecurserDB) Delete(ctx context.Context, userID string) error {
	return Recurser{}, nil
}

func (m *MockRecurserDB) ListPairingTomorrow(ctx context.Context) ([]Recurser, error) {
	return Recurser{}, nil
}

func (m *MockRecurserDB) ListSkippingTomorrow(ctx context.Context) ([]Recurser, error) {
	return Recurser{}, nil
}

func (m *MockRecurserDB) SetSkippingTomorrow(ctx context.Context, userID string) error {
	return Recurser{}, nil
}

func (m *MockRecurserDB) UnsetSkippingTomorrow(ctx context.Context, userID string) error {
	return Recurser{}, nil
}

// Zulip Auth Token Lookup

type APIAuthDB interface {
	GetAPIAuthKey(ctx context.Context) (string, error)
}

type FirestoreAPIAuthDB struct {
	client *firestore.Client
}

func (f *FirestoreAPIAuthDB) GetAPIAuthKey(ctx context.Context) (string, error) {
	doc, err := f.client.Collection("botauth").Doc("token").Get(ctx)
	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
		return "", err
	}

	token := doc.Data()
	return token["value"].(string), nil
}

type MockAPIAuthDB struct{}

func (f *MockAPIAuthDB) GetAPIAuthKey(ctx context.Context) (string, error) {
	return "test", nil
}
