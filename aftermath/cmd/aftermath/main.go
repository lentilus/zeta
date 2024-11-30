package main

import (
	"aftermath/internal/api"
	"aftermath/internal/bibliography"
	"aftermath/internal/cache"
	"aftermath/internal/database"
	"aftermath/internal/scheduler"
	"flag"
	"log"
	"time"
)

func main() {
	port := flag.Int("port", 1234, "The port on which the server will listen.")
	root := flag.String("root", "/home/lentilus/typstest/", "The root of the zettel kasten.")
	cachefile := flag.String(
		"cache",
		"/home/lentilus/typstest/.state/index.sqlite",
		"The full path to the sqlite cache.",
	)
	bibfile := flag.String(
		"bib",
		"/home/lentilus/typstest/.state/index.yaml",
		"The full path to the sqlite cache.",
	)

	flag.Parse()

	// Initialize read-write db for cache update
	rwDB, err := database.NewDB(*cachefile)
	if err != nil {
		log.Fatal(err)
	}
	defer rwDB.Close()

	// Initialize read-only db for api
	roDB, err := database.NewReadonlyDB(*cachefile, 1000)
	if err != nil {
		log.Fatal(err)
	}
	defer roDB.Close()

	// Create and start new scheduler
	s := scheduler.NewScheduler(10)
	go s.RunScheduler()
	defer s.StopScheduler()

	b := &bibliography.Bibliography{
		Path:     *bibfile,
		DB:       rwDB,
		Checksum: []byte{},
	}

	// Schedule incremental updates every 5 minutes
	zk := cache.NewZettelkasten(*root, rwDB, b)
	go func() {
		t := scheduler.Task{Name: "Incremental Cache Update", Execute: zk.UpdateIncremental}
		s.SchedulePeriodicTask(5*time.Minute, t)
	}()

	// Initialize the JSON-RPC api
	index := api.NewIndex(roDB, zk, s)
	server := api.NewJSONRPCServer(&index, "API", *port)
	server.Start()
}
