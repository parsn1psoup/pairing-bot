package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/firestore"
)

// It's alive! The application starts here.
func main() {

	// setting up database connection: 2 clients encapsulated into PairingLogic struct

	ctx := context.Background()

	rc, err := firestore.NewClient(ctx, "pairing-bot-284823")
	if err != nil {
		log.Panic(err)
	}
	defer rc.Close()

	ac, err := firestore.NewClient(ctx, "pairing-bot-284823")
	if err != nil {
		log.Panic(err)
	}
	defer ac.Close()

	rdb := &FirestoreRecurserDB{
		client: rc,
	}

	adb := &FirestoreAPIAuthDB{
		client: ac,
	}

	ur := &zulipUserRequest{}

	un := &zulipUserNotification{
		botUsername: "pairing-bot@recurse.zulipchat.com",
		zulipAPIURL: "https://recurse.zulipchat.com/api/v1/messages",
	}

	pl := &PairingLogic{
		rdb: rdb,
		adb: adb,
		ur:  ur,
		un:  un,
	}

	http.HandleFunc("/", http.NotFound)           // will this handle anything that's not defined?
	http.HandleFunc("/webhooks", pl.handle)       // from zulip
	http.HandleFunc("/match", pl.match)           // from GCP
	http.HandleFunc("/endofbatch", pl.endofbatch) // manually triggered

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	if m, ok := os.LookupEnv("PB_MAINT"); ok {
		if m == "true" {
			maintenanceMode = true
		}
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
