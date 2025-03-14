package main

import (
	"log"
	"net/http"
	"time"

	"tjens23.dk/K8sPulse/src/database"
	"tjens23.dk/K8sPulse/src/metrics"
)

func main() {

	server := metrics.NewMetricsServer()
	http.HandleFunc("/metrics/", server.MetricsHandler)

	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	database.Connect()
	log.Println("Metrics server started on :8080")
	log.Fatal(srv.ListenAndServe())
}
