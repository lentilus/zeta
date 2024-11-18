package main

import (
	"aftermath/internal/api"
	"aftermath/internal/cache"
	"flag"
)

func main() {
	_ = flag.Int("port", 8080, "The port on which the server will listen.")
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
	// db, err := database.NewReadonlyDB(*cachefile, 1000)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer db.Close()

	// Initialize the HTTP server with database dependency
	api.StartServer()
}
