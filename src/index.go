package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"tjens23.dk/K8sPulse/src/database"
	"tjens23.dk/K8sPulse/src/metrics"
	"tjens23.dk/K8sPulse/src/routes"
)

func main() {
	app := fiber.New()
	routes.GetRoutes(app)
	server := metrics.NewMetricsServer()
	app.Get("/metrics/*", server.MetricsHandler)

	database.Connect()
	log.Println("Metrics server started on :8080")

	if err := app.Listen(":8080"); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}
