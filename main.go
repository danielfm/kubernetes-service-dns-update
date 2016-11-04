package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var (
	dryRun       = false
	namespace    = ""
	syncInterval = 30
)

func main() {
	flag.BoolVar(&dryRun, "dry-run", dryRun, "Don't actually commit the changes to DNS records, just print out what we would have done.")
	flag.IntVar(&syncInterval, "sync-interval", syncInterval, "Sync interval in seconds.")
	flag.StringVar(&namespace, "namespace", namespace, "Namespace to be monitored.")

	flag.Parse()

	log.Println("DNS update service started.")

	doneChan := make(chan struct{})
	var wg sync.WaitGroup

	log.Printf("Watching services in '%s' namespace (every %d secs).\n", namespace, syncInterval)
	wg.Add(1)

	WatchServices(syncInterval, doneChan, &wg)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-signalChan:
			log.Printf("Shutdown signal received, exiting...")
			close(doneChan)
			wg.Wait()
			os.Exit(0)
		}
	}
}
