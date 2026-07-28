package main

import (
	"flag"
	"log"
	"net/http"

	clustermodule "eden-microservice/apps/cluster/module"
)

func main() {
	address := flag.String("http", ":8900", "cluster-management module HTTP listen address")
	dataDir := flag.String("data-dir", "./data", "cluster-management module data directory")
	flag.Parse()

	state := clustermodule.NewRuntimeState(*dataDir)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		if state == nil {
			http.Error(w, "cluster management unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("OK"))
	})
	log.Printf("cluster-management module listening on %s", *address)
	log.Fatal(http.ListenAndServe(*address, mux))
}
