package main

import (
	"log"
	"time"

	"github.com/wiraphatys/intania888/internal/model"
	"github.com/wiraphatys/intania888/pkg/config"
	"github.com/wiraphatys/intania888/pkg/database"
)

func main() {
	cfg := config.GetConfig()
	db := database.NewGormDatabase(cfg)

	if err := db.AutoMigrate(&model.User{}, &model.Role{}); err != nil {
		log.Fatalf("Error during migration: %v", err)
	}

	roles := []model.Role{
		{ID: "user", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	if err := db.Create(&roles).Error; err != nil {
		log.Printf("Error creating role: %v", err)
	}
}
