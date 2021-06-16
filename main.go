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

	// TODO context
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

	pl := &PairingLogic{
		rdb: rdb,
		adb: adb,
		ur:  ur,
	}

	http.HandleFunc("/", nope)                    // will this handle anything that's not defined?
	http.HandleFunc("/webhooks", pl.handle)       // from zulip
	http.HandleFunc("/match", pl.match)           // from GCP
	http.HandleFunc("/endofbatch", pl.endofbatch) // manually triggered

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
