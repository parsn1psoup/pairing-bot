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

	// setting up database connection
	ctx := context.Background()

	client, err := firestore.NewClient(ctx, "pairing-bot-284823")
	if err != nil {
		log.Panic(err)
	}
	defer client.Close()

	rdb := &FirestoreRecurserDB{
		client: client,
	}

	http.HandleFunc("/", nope)
	http.HandleFunc("/webhooks", rdb.handle)
	http.HandleFunc("/match", rdb.match)
	http.HandleFunc("/endofbatch", rdb.endofbatch)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	log.Printf("Listening on port %s", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}
