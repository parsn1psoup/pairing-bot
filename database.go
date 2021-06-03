package main

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
)

type Recurser struct {
	id                 string
	name               string
	email              string
	isSkippingTomorrow bool
	schedule           map[string]interface{}
	isSubscribed       bool
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

// Subscriber List Lookup

type RecurserDB interface {
	GetByUserID(ctx context.Context, userID string) (Recurser, error)
	Set(ctx context.Context, userID string, recurser Recurser) error
	Delete(ctx context.Context, userID string) error
	ListPairingTomorrow(ctx context.Context) ([]Recurser, error)
	ListSkippingTomorrow(ctx context.Context) ([]Recurser, error)
	SetSkippingTomorrow(ctx context.Context, userID string) error
	UnsetSkippingTomorrow(ctx context.Context, recurser Recurser) error
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
	// this gets the time from system time, which is UTC
	// on app engine (and most other places). This works
	// fine for us in NYC, but might not if pairing bot
	// were ever running in another time zone
	today := strings.ToLower(time.Now().Weekday().String())

	var recursersList []Recurser
	var r Recurser

	// ok this is how we have to get all the recursers. it's weird.
	// this query returns an iterator, and then we have to use firestore
	// magic to iterate across the results of the query and store them
	// into our 'recursersList' variable which is a slice of map[string]interface{}
	iter := f.client.Collection("recursers").Where("isSkippingTomorrow", "==", false).Where("schedule."+today, "==", true).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		r = MapToStruct(doc.Data())

		recursersList = append(recursersList, r)
	}

	return recursersList, nil
}

func (f *FirestoreRecurserDB) ListSkippingTomorrow(ctx context.Context) ([]Recurser, error) {

	var skippersList []Recurser
	var r Recurser

	iter = f.client.Collection("recursers").Where("isSkippingTomorrow", "==", true).Documents(ctx)
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		r = MapToStruct(doc.Data())

		skippersList = append(skippersList, r)
	}
	return skippersList, nil
}

func (f *FirestoreRecurserDB) SetSkippingTomorrow(ctx context.Context, userID string) error {
	return nil // TODO
}

func (f *FirestoreRecurserDB) UnsetSkippingTomorrow(ctx context.Context, recurser Recurser) error {

	r := MapToStruct(recurser)
	r["isSkippingTomorrow"] = false

	_, err := f.client.Collection("recursers").Doc(r["id"].(string)).Set(ctx, r, firestore.MergeAll)
	if err != nil {
		return err
	}
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

// Token Lookups

type APIAuthDB interface {
	GetKey(ctx context.Context, col, doc string) (string, error)
}

type FirestoreAPIAuthDB struct {
	client *firestore.Client
}

func (f *FirestoreAPIAuthDB) GetKey(ctx context.Context, col, doc string) (string, error) {
	doc, err := f.client.Collection(col).Doc(doc).Get(ctx)
	if err != nil {
		log.Println("Something weird happened trying to read the auth token from the database")
		return "", err
	}

	token := doc.Data()
	return token["value"].(string), nil
}

type MockAPIAuthDB struct{}

func (f *MockAPIAuthDB) GetKey(ctx context.Context, col, doc string) (string, error) {
	return "test", nil
}
