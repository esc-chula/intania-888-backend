package main

import (
	"log"
	"time"

	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/database"
	"github.com/esc-chula/intania-888-backend/utils/constant"
	"github.com/google/uuid"
)

type matchInfo struct {
	teamA, teamB, sportType string
	startTime               string
	duration                time.Duration
}

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
		&model.DailyReward{},
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
			Id:    constant.FOOTBALL_MALE_JR,
			Title: "ฟุตบอล ชาย ปี1",
		},
		{
			Id:    constant.FOOTBALL_MALE_SR,
			Title: "ฟุตบอล ชาย ปี2-4",
		},
		{
			Id:    constant.BASKETBALL_MALE_JR,
			Title: "บาสเกตบอล ชาย ปี1",
		},
		{
			Id:    constant.BASKETBALL_MALE_SR,
			Title: "บาสเกตบอล ชาย ปี2-4",
		},
		{
			Id:    constant.BASKETBALL_FEMALE_ALL,
			Title: "บาสเกตบอล หญิง ทุกชั้นปี",
		},
		{
			Id:    constant.VOLLEYBALL_MALE_ALL,
			Title: "วอลเลย์บอล ชาย ทุกชั้นปี",
		},
		{
			Id:    constant.VOLLEYBALL_FEMALE_ALL,
			Title: "วอลเลย์บอล หญิง ทุกชั้นปี",
		},
		{
			Id:    constant.CHAIRBALL_FEMALE_JR,
			Title: "แชร์บอล หญิง ปี1",
		},
		{
			Id:    constant.CHAIRBALL_FEMALE_SR,
			Title: "แชร์บอล หญิง ปี2-4",
		},
	}

	// Define color teams
	colorTeams := map[string]model.Color{
		"VIOLET": {Id: "VIOLET", Title: "สีม่วง"},
		"BLUE":   {Id: "BLUE", Title: "สีน้ำเงิน"},
		"GREEN":  {Id: "GREEN", Title: "สีเขียว"},
		"PINK":   {Id: "PINK", Title: "สีชมพู"},
		"ORANGE": {Id: "ORANGE", Title: "สีส้ม"},
		"YELLOW": {Id: "YELLOW", Title: "สีเหลือง"},
	}

	// Define match schedule for the entire month
	matchSchedule := []struct {
		Date    time.Time
		Matches []matchInfo
	}{
		{
			Date: time.Date(2024, 10, 10, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "ORANGE", teamB: "PINK", sportType: constant.CHAIRBALL_FEMALE_JR, startTime: "17:30", duration: 2 * time.Hour},
				{teamA: "ORANGE", teamB: "PINK", sportType: constant.CHAIRBALL_FEMALE_SR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.VOLLEYBALL_FEMALE_ALL, startTime: "18:45", duration: 2 * time.Hour},
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "20:00", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 11, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "VIOLET", teamB: "BLUE", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "BLUE", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "GREEN", teamB: "ORANGE", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:30", duration: 2 * time.Hour},
				{teamA: "GREEN", teamB: "ORANGE", sportType: constant.BASKETBALL_MALE_JR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "GREEN", teamB: "ORANGE", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:30", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 15, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "GREEN", teamB: "ORANGE", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 2 * time.Hour},
				{teamA: "GREEN", teamB: "ORANGE", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "BLUE", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:30", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "BLUE", sportType: constant.BASKETBALL_MALE_JR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "BLUE", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:30", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 16, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.CHAIRBALL_FEMALE_JR, startTime: "16:30", duration: 2 * time.Hour},
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.CHAIRBALL_FEMALE_SR, startTime: "17:00", duration: 2 * time.Hour},
				{teamA: "ORANGE", teamB: "PINK", sportType: constant.VOLLEYBALL_FEMALE_ALL, startTime: "17:45", duration: 2 * time.Hour},
				{teamA: "ORANGE", teamB: "PINK", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "19:00", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 17, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "VIOLET", teamB: "YELLOW", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "YELLOW", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "PINK", teamB: "GREEN", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:30", duration: 2 * time.Hour},
				{teamA: "PINK", teamB: "GREEN", sportType: constant.BASKETBALL_MALE_JR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "PINK", teamB: "GREEN", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:30", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 18, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "ORANGE", teamB: "GREEN", sportType: constant.CHAIRBALL_FEMALE_JR, startTime: "16:30", duration: 2 * time.Hour},
				{teamA: "ORANGE", teamB: "GREEN", sportType: constant.CHAIRBALL_FEMALE_SR, startTime: "17:00", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 21, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "PINK", teamB: "GREEN", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 2 * time.Hour},
				{teamA: "PINK", teamB: "GREEN", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "YELLOW", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:30", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "YELLOW", sportType: constant.BASKETBALL_MALE_JR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "YELLOW", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:30", duration: 2 * time.Hour},
				{teamA: "BLUE", teamB: "VIOLET", sportType: constant.VOLLEYBALL_FEMALE_ALL, startTime: "16:45", duration: 2 * time.Hour},
				{teamA: "BLUE", teamB: "VIOLET", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "17:00", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 22, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "VIOLET", teamB: "BLUE", sportType: constant.CHAIRBALL_FEMALE_JR, startTime: "17:30", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "BLUE", sportType: constant.CHAIRBALL_FEMALE_SR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "GREEN", teamB: "ORANGE", sportType: constant.VOLLEYBALL_FEMALE_ALL, startTime: "18:45", duration: 2 * time.Hour},
				{teamA: "GREEN", teamB: "ORANGE", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "20:00", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 24, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 2 * time.Hour},
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "ORANGE", teamB: "PINK", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:30", duration: 2 * time.Hour},
				{teamA: "ORANGE", teamB: "PINK", sportType: constant.BASKETBALL_MALE_JR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "ORANGE", teamB: "PINK", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:30", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 28, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "GREEN", teamB: "PINK", sportType: constant.CHAIRBALL_FEMALE_JR, startTime: "16:30", duration: 2 * time.Hour},
				{teamA: "GREEN", teamB: "PINK", sportType: constant.CHAIRBALL_FEMALE_SR, startTime: "17:00", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "YELLOW", sportType: constant.VOLLEYBALL_FEMALE_ALL, startTime: "17:45", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "YELLOW", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "19:00", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 29, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "ORANGE", teamB: "BLUE", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 2 * time.Hour},
				{teamA: "ORANGE", teamB: "BLUE", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:30", duration: 2 * time.Hour},
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.BASKETBALL_MALE_JR, startTime: "18:00", duration: 2 * time.Hour},
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:30", duration: 2 * time.Hour},
			},
		},
		{
			Date: time.Date(2024, 10, 30, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "VIOLET", teamB: "YELLOW", sportType: constant.CHAIRBALL_FEMALE_JR, startTime: "16:30", duration: 2 * time.Hour},
				{teamA: "VIOLET", teamB: "YELLOW", sportType: constant.CHAIRBALL_FEMALE_SR, startTime: "17:00", duration: 2 * time.Hour},
				{teamA: "PINK", teamB: "GREEN", sportType: constant.VOLLEYBALL_FEMALE_ALL, startTime: "17:45", duration: 2 * time.Hour},
				{teamA: "PINK", teamB: "GREEN", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "19:00", duration: 2 * time.Hour},
			},
		},
	}

	// Generate matches
	var matchesMock []model.Match
	for _, day := range matchSchedule {
		for _, m := range day.Matches {
			startTime, _ := time.Parse("15:04", m.startTime)
			matchDateTime := time.Date(day.Date.Year(), day.Date.Month(), day.Date.Day(),
				startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)
			teamA := colorTeams[m.teamA].Id
			teamB := colorTeams[m.teamB].Id
			match := model.Match{
				Id:        uuid.NewString(),
				TeamA_Id:  &teamA,
				TeamB_Id:  &teamB,
				TypeId:    m.sportType,
				StartTime: matchDateTime,
				EndTime:   matchDateTime.Add(m.duration),
			}
			matchesMock = append(matchesMock, match)
		}
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
	if err := db.Create(&matchesMock).Error; err != nil {
		log.Printf("Error creating matches: %v", err)
	}

	log.Println("migration successful.")
}
