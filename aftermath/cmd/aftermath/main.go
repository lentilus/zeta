package main

import (
	"aftermath/internal/api"
	"aftermath/internal/cache"
	"aftermath/internal/database"
	"flag"
	"log"
)

func main() {
	port := flag.Int("port", 1234, "The port on which the server will listen.")
	root := flag.String("root", "/home/lentilus/typstest/", "The root of the zettel kasten.")
	cachefile := flag.String(
		"cache",
		"/home/lentilus/typstest/lul.sqlite",
		"The full path to the sqlite cache.",
	)

	flag.Parse()

	// Start cache generation in the background immediately
	db, err := database.NewDB(*cachefile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	kasten := cache.NewZettelkasten(*root, db)
	kasten.UpdateIncremental()

	// Initialize the database
	roDB, err := database.NewReadonlyDB(*cachefile, 1000)
	if err != nil {
		log.Fatal(err)
	}
	defer roDB.Close()

	// Initialize the HTTP server with database dependency
	cacheapi := api.NewCache(roDB)
	server := api.NewJSONRPCServer(&cacheapi, "API", *port)
	log.Fatal(server.Start())
}
