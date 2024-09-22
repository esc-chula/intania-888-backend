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

	if err := db.AutoMigrate(
		&model.User{},
		&model.Role{},
		&model.SportType{},
		&model.Color{},
		&model.IntaniaGroup{},
		&model.BillHead{},
		&model.BillLine{},
		&model.Match{},
		&model.GroupHead{},
		&model.GroupLine{},
	); err != nil {
		log.Fatalf("Error during migration: %v", err)
	}

	roles := []model.Role{
		{ID: "USER", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "ADMIN", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	violet := model.Color{
		Id:    "VIOLET",
		Title: "สีม่วง",
		Members: []model.IntaniaGroup{
			{Id: "A", ColorId: "VIOLET"},
			{Id: "C", ColorId: "VIOLET"},
			{Id: "F", ColorId: "VIOLET"},
		},
	}
	blue := model.Color{
		Id:    "BLUE",
		Title: "สีน้ำเงิน",
		Members: []model.IntaniaGroup{
			{Id: "DOG", ColorId: "BLUE"},
			{Id: "N", ColorId: "BLUE"},
			{Id: "R", ColorId: "BLUE"},
		},
	}
	green := model.Color{
		Id:    "GREEN",
		Title: "สีเขียว",
		Members: []model.IntaniaGroup{
			{Id: "H", ColorId: "GREEN"},
			{Id: "S", ColorId: "GREEN"},
			{Id: "T", ColorId: "GREEN"},
		},
	}
	pink := model.Color{
		Id:    "PINK",
		Title: "สีชมพู",
		Members: []model.IntaniaGroup{
			{Id: "E", ColorId: "PINK"},
			{Id: "L", ColorId: "PINK"},
			{Id: "P", ColorId: "PINK"},
		},
	}
	orange := model.Color{
		Id:    "ORANGE",
		Title: "สีส้ม",
		Members: []model.IntaniaGroup{
			{Id: "J", ColorId: "ORANGE"},
			{Id: "K", ColorId: "ORANGE"},
			{Id: "M", ColorId: "ORANGE"},
		},
	}
	yellow := model.Color{
		Id:    "YELLOW",
		Title: "สีเหลือง",
		Members: []model.IntaniaGroup{
			{Id: "B", ColorId: "YELLOW"},
			{Id: "G", ColorId: "YELLOW"},
			{Id: "Q", ColorId: "YELLOW"},
		},
	}
	colors := []model.Color{}
	colors = append(colors, violet, blue, green, pink, orange, yellow)

	sportTypes := []model.SportType{
		{
			Id:    "FOOTBALL_MALE_JR",
			Title: "ฟุตบอล ชาย ปี1",
		},
		{
			Id:    "FOOTBALL_MALE_SR",
			Title: "ฟุตบอล ชาย ปี2-4",
		},
		{
			Id:    "BASKETBALL_MALE_JR",
			Title: "บาสเกตบอล ชาย ปี1",
		},
		{
			Id:    "BASKETBALL_MALE_SR",
			Title: "บาสเกตบอล ชาย ปี2-4",
		},
		{
			Id:    "BASKETBALL_FEMALE_ALL",
			Title: "บาสเกตบอล หญิง ทุกชั้นปี",
		},
		{
			Id:    "VOLLEYBALL_MALE_JR",
			Title: "วอลเลย์บอล ชาย ปี1",
		},
		{
			Id:    "VOLLEYBALL_MALE_SR",
			Title: "วอลเลย์บอล ชาย ปี2-4",
		},
		{
			Id:    "VOLLEYBALL_FEMALE_ALL",
			Title: "วอลเลย์บอล หญิง ทุกชั้นปี",
		},
		{
			Id:    "CHAIRBALL_FEMALE_ALL",
			Title: "แชร์บอล หญิง ทุกชั้นปี",
		},
	}

	if err := db.Create(&roles).Error; err != nil {
		log.Printf("Error creating roles: %v", err)
	}
	if err := db.Create(&colors).Error; err != nil {
		log.Printf("Error creating colors: %v", err)
	}
	if err := db.Create(&sportTypes).Error; err != nil {
		log.Printf("Error creating sport_types: %v", err)
	}

	log.Println("migration successful.")
}
