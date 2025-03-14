package database

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"tjens23.dk/K8sPulse/src/database/models"
)

var DB *gorm.DB

func Connect() {
	dsn := "user=tjens23 password=1234 dbname=k8spulse host=localhost port=5432 sslmode=disable"
	connection, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("Could not connect to the database")
	}

	connection.AutoMigrate(&models.User{})
	DB = connection
}
