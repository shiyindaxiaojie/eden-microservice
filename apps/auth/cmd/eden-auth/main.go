package main

import (
	"flag"
	"log"
	"net/http"

	authmodule "eden-microservice/apps/auth/module"
)

func main() {
	address := flag.String("http", ":8800", "permission-control module HTTP listen address")
	dataDir := flag.String("data-dir", "./data", "permission-control module data directory")
	flag.Parse()

	container := authmodule.NewContainer(*dataDir)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		if container.Store == nil || container.Authenticator == nil {
			http.Error(w, "permission control unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("OK"))
	})
	log.Printf("permission-control module listening on %s", *address)
	log.Fatal(http.ListenAndServe(*address, mux))
}
