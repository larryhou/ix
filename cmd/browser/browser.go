package main

import (
	"context"
	"github.com/grandcat/zeroconf"
	"log"
	"time"
)

func main() {
	// Discover all services on the network (e.g. _workstation._tcp)
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		log.Fatalln("Failed to initialize resolver:", err.Error())
	}

	var a any
	switch a.(type) {
	case nil: log.Printf(`NIL VALUE FOUND`)
	}

	rootCtx, rootCancel := context.WithCancel(context.Background())

	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			log.Printf("%+v\n", entry)
			rootCancel()
		}
		log.Println("No more entries.")
	}(entries)

	ctx, cancel := context.WithTimeout(rootCtx, time.Second*15)
	defer cancel()
	err = resolver.Browse(ctx, "_remoted._tcp", "local.", entries)
	if err != nil {
		log.Fatalln("Failed to browse:", err.Error())
	}

	<-ctx.Done()
}