package main

import (
	"flag"
	"log"
	"net/http"

	registrymodule "eden-microservice/apps/registry/module"
)

func main() {
	address := flag.String("http", ":8500", "registry module HTTP listen address")
	dataDir := flag.String("data-dir", "./data", "registry module data directory")
	flag.Parse()

	container := registrymodule.NewContainer(*dataDir, nil, nil, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		if container.State == nil || container.Service == nil {
			http.Error(w, "registry unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("OK"))
	})
	log.Printf("registry module listening on %s", *address)
	log.Fatal(http.ListenAndServe(*address, mux))
}
