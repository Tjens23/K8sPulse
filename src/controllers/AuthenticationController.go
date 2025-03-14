package controllers

import "github.com/gofiber/fiber/v2"

func GetHello(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "Hello, World!",
		"status":  fiber.StatusOK,
	})
}
