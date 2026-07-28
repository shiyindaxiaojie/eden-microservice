package main

import (
	"flag"
	"log"
	"net/http"

	configmodule "eden-microservice/apps/config/module"
)

func main() {
	address := flag.String("http", ":8600", "config module HTTP listen address")
	dataDir := flag.String("data-dir", "./data", "config module data directory")
	flag.Parse()

	container, err := configmodule.NewContainer(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer container.Service.Close()

	mux := http.NewServeMux()
	configmodule.NewNacosHTTPAdapter(container.Service).RegisterRoutes(mux, "/nacos", func(next http.Handler) http.Handler { return next })
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("OK")) })
	log.Printf("config module listening on %s", *address)
	log.Fatal(http.ListenAndServe(*address, mux))
}
