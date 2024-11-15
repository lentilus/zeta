package main

import (
	"aftermath/internal/cache"
	"aftermath/internal/database"
	"aftermath/internal/server"
	"flag"
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := flag.Int("port", 8080, "The port on which the server will listen.")
	root := flag.String("root", "/home/lentilus/typstest/", "The root of the zettel kasten.")
	cachefile := flag.String(
		"cache",
		"/home/lentilus/typstest/lul.sqlite",
		"The full path to the sqlite cache.",
	)

	flag.Parse()

	// Start cache generation in the background immediately
	kasten := cache.NewZettelkasten(*root, *cachefile)
	kasten.UpdateIncremental()

	// Initialize the database
	db, err := database.NewDB(*cachefile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Initialize the HTTP server with database dependency
	srv := server.NewServer(db)

	log.Printf("Server running at http://localhost:%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), srv.Router()))
}
