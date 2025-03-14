package database

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"tjens23.dk/K8sPulse/src/database/models"
)

var DB *gorm.DB

func Connect() {

	if err := godotenv.Load(".env"); err != nil {
		log.Panic("No .env file found")
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_SSLMODE"),
	)

	log.Println("Connecting to database:",
		strings.Replace(dsn, os.Getenv("DB_PASSWORD"), "****", 1))

	connection, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Panic("Database connection error: ", err)
	}

	// Test the connection
	sqlDB, err := connection.DB()
	if err != nil {
		log.Panic("Failed to get database instance: ", err)
	}

	err = sqlDB.Ping()
	if err != nil {
		log.Panic("Failed to ping database: ", err)
	}

	log.Println("Successfully connected to database")

	connection.AutoMigrate(&models.User{})
	DB = connection
}
