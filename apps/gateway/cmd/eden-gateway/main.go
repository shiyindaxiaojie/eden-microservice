package main

import (
	"flag"
	"log"
	"net/http"

	gatewaymodule "eden-microservice/apps/gateway/module"
)

func main() {
	address := flag.String("http", ":8700", "gateway control module HTTP listen address")
	dataDir := flag.String("data-dir", "./data", "gateway module data directory")
	flag.Parse()

	container, err := gatewaymodule.NewContainer(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer container.Service.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("OK")) })
	log.Printf("gateway module listening on %s", *address)
	log.Fatal(http.ListenAndServe(*address, mux))
}
