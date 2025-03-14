package routes

import (
	"github.com/gofiber/fiber/v2"
	"tjens23.dk/K8sPulse/src/controllers"
)

func GetRoutes(app *fiber.App) {
	app.Get("/hello", controllers.GetHello)
	app.Post("/register", controllers.Register)
}
