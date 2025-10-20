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
		&model.StealToken{},
		&model.GroupStage{},
		&model.MineGame{},
		&model.MineGameHistory{},
	); err != nil {
		log.Fatalf("Error during migration: %v", err)
	}

	// DELETE ALL EXISTING MATCH-RELATED DATA (preserve user data and group assignments)
	log.Println("Deleting existing match-related data...")

	// Delete in correct order to respect foreign key constraints
	// NOTE: We do NOT delete intania_groups or colors - those are permanent
	if err := db.Exec("DELETE FROM mine_game_histories").Error; err != nil {
		log.Printf("Warning: Error deleting mine_game_histories: %v", err)
	}
	if err := db.Exec("DELETE FROM mine_games").Error; err != nil {
		log.Printf("Warning: Error deleting mine_games: %v", err)
	}
	if err := db.Exec("DELETE FROM group_stages").Error; err != nil {
		log.Printf("Warning: Error deleting group_stages: %v", err)
	}
	if err := db.Exec("DELETE FROM bill_lines").Error; err != nil {
		log.Printf("Warning: Error deleting bill_lines: %v", err)
	}
	if err := db.Exec("DELETE FROM bill_heads").Error; err != nil {
		log.Printf("Warning: Error deleting bill_heads: %v", err)
	}
	if err := db.Exec("DELETE FROM group_lines").Error; err != nil {
		log.Printf("Warning: Error deleting group_lines: %v", err)
	}
	if err := db.Exec("DELETE FROM group_heads").Error; err != nil {
		log.Printf("Warning: Error deleting group_heads: %v", err)
	}
	if err := db.Exec("DELETE FROM matches").Error; err != nil {
		log.Printf("Warning: Error deleting matches: %v", err)
	}
	if err := db.Exec("DELETE FROM sport_types").Error; err != nil {
		log.Printf("Warning: Error deleting sport_types: %v", err)
	}

	log.Println("Existing match-related data deleted successfully.")

	roles := []model.Role{
		{ID: "USER", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		{ID: "ADMIN", CreatedAt: time.Now(), UpdatedAt: time.Now()},
	}

	// Updated color assignments for 2025
	violet := model.Color{
		Id:    "VIOLET",
		Title: "สีม่วง",
		Members: []model.IntaniaGroup{
			{Id: "DOG", ColorId: "VIOLET"},
			{Id: "J", ColorId: "VIOLET"},
			{Id: "R", ColorId: "VIOLET"},
		},
	}
	blue := model.Color{
		Id:    "BLUE",
		Title: "สีฟ้า",
		Members: []model.IntaniaGroup{
			{Id: "E", ColorId: "BLUE"},
			{Id: "K", ColorId: "BLUE"},
			{Id: "N", ColorId: "BLUE"},
		},
	}
	green := model.Color{
		Id:    "GREEN",
		Title: "สีเขียว",
		Members: []model.IntaniaGroup{
			{Id: "B", ColorId: "GREEN"},
			{Id: "C", ColorId: "GREEN"},
			{Id: "M", ColorId: "GREEN"},
		},
	}
	pink := model.Color{
		Id:    "PINK",
		Title: "สีชมพู",
		Members: []model.IntaniaGroup{
			{Id: "G", ColorId: "PINK"},
			{Id: "H", ColorId: "PINK"},
			{Id: "T", ColorId: "PINK"},
		},
	}
	orange := model.Color{
		Id:    "ORANGE",
		Title: "สีส้ม",
		Members: []model.IntaniaGroup{
			{Id: "P", ColorId: "ORANGE"},
			{Id: "Q", ColorId: "ORANGE"},
			{Id: "S", ColorId: "ORANGE"},
		},
	}
	yellow := model.Color{
		Id:    "YELLOW",
		Title: "สีเหลือง",
		Members: []model.IntaniaGroup{
			{Id: "A", ColorId: "YELLOW"},
			{Id: "F", ColorId: "YELLOW"},
			{Id: "L", ColorId: "YELLOW"},
		},
	}
	colors := []model.Color{}
	colors = append(colors, violet, blue, green, pink, orange, yellow)

	sportTypes := []model.SportType{
		{
			Id:    constant.BASKETBALL_MALE_JR,
			Title: "บาสเก็ตบอลชาย ปี 1",
		},
		{
			Id:    constant.BASKETBALL_MALE_SR,
			Title: "บาสเก็ตบอลชาย ปี 2-4",
		},
		{
			Id:    constant.BASKETBALL_FEMALE_ALL,
			Title: "บาสเก็ตบอลหญิง รวมชั้นปี",
		},
		{
			Id:    constant.VOLLEYBALL_MALE_ALL,
			Title: "วอลเลย์บอลชาย รวมชั้นปี",
		},
		{
			Id:    constant.VOLLEYBALL_FEMALE_ALL,
			Title: "วอลเลย์บอลหญิง รวมชั้นปี",
		},
		{
			Id:    constant.FOOTBALL_MALE_JR,
			Title: "ฟุตบอลชาย ปี 1",
		},
		{
			Id:    constant.FOOTBALL_MALE_SR,
			Title: "ฟุตบอลชาย ปี 2-4",
		},
		{
			Id:    constant.CHAIRBALL_FEMALE_ALL,
			Title: "แชร์บอลหญิง รวมชั้นปี",
		},
		{
			Id:    constant.RUNNING,
			Title: "วิ่งเปี้ยว",
		},
		{
			Id:    constant.TUG_OF_WAR,
			Title: "ชักเย่อ",
		},
	}

	// Define color teams
	colorTeams := map[string]model.Color{
		"VIOLET": {Id: "VIOLET", Title: "สีม่วง"},
		"BLUE":   {Id: "BLUE", Title: "สีฟ้า"},
		"GREEN":  {Id: "GREEN", Title: "สีเขียว"},
		"PINK":   {Id: "PINK", Title: "สีชมพู"},
		"ORANGE": {Id: "ORANGE", Title: "สีส้ม"},
		"YELLOW": {Id: "YELLOW", Title: "สีเหลือง"},
	}

	// Define match schedule for October-November 2025
	// Times are Bangkok local time but stored as UTC (subtract 7 hours)
	// This way server's time.Now() (UTC) can be compared directly with match times
	matchSchedule := []struct {
		Date    time.Time
		Matches []matchInfo
	}{
		{
			// 17 October 2025 (stored as UTC, so 17:00 Bangkok = 10:00 UTC)
			Date: time.Date(2025, 10, 17, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "BLUE", teamB: "ORANGE", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:00", duration: 40 * time.Minute},
				{teamA: "BLUE", teamB: "ORANGE", sportType: constant.BASKETBALL_MALE_JR, startTime: "17:40", duration: 60 * time.Minute},
				{teamA: "BLUE", teamB: "ORANGE", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:40", duration: 60 * time.Minute},
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 60 * time.Minute},
				{teamA: "BLUE", teamB: "YELLOW", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 60 * time.Minute},
			},
		},
		{
			// 20 October 2025
			Date: time.Date(2025, 10, 20, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "PINK", teamB: "GREEN", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:00", duration: 40 * time.Minute},
				{teamA: "PINK", teamB: "GREEN", sportType: constant.BASKETBALL_MALE_JR, startTime: "17:40", duration: 60 * time.Minute},
				{teamA: "PINK", teamB: "GREEN", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:40", duration: 60 * time.Minute},
				{teamA: "PINK", teamB: "ORANGE", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 60 * time.Minute},
				{teamA: "PINK", teamB: "ORANGE", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 60 * time.Minute},
			},
		},
		{
			// 21 October 2025
			Date: time.Date(2025, 10, 21, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "BLUE", teamB: "VIOLET", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:00", duration: 40 * time.Minute},
				{teamA: "BLUE", teamB: "VIOLET", sportType: constant.BASKETBALL_MALE_JR, startTime: "17:40", duration: 60 * time.Minute},
				{teamA: "BLUE", teamB: "VIOLET", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:40", duration: 60 * time.Minute},
				{teamA: "BLUE", teamB: "GREEN", sportType: constant.VOLLEYBALL_FEMALE_ALL, startTime: "17:00", duration: 40 * time.Minute},
				{teamA: "YELLOW", teamB: "ORANGE", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "17:40", duration: 50 * time.Minute},
			},
		},
		{
			// 27 October 2025
			Date: time.Date(2025, 10, 27, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "PINK", teamB: "YELLOW", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:00", duration: 40 * time.Minute},
				{teamA: "PINK", teamB: "YELLOW", sportType: constant.BASKETBALL_MALE_JR, startTime: "17:40", duration: 60 * time.Minute},
				{teamA: "PINK", teamB: "YELLOW", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:40", duration: 60 * time.Minute},
				{teamA: "BLUE", teamB: "GREEN", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 60 * time.Minute},
				{teamA: "BLUE", teamB: "GREEN", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 60 * time.Minute},
				{teamA: "YELLOW", teamB: "PINK", sportType: constant.CHAIRBALL_FEMALE_ALL, startTime: "16:40", duration: 40 * time.Minute},
				{teamA: "ORANGE", teamB: "GREEN", sportType: constant.CHAIRBALL_FEMALE_ALL, startTime: "17:30", duration: 40 * time.Minute},
			},
		},
		{
			// 28 October 2025
			Date: time.Date(2025, 10, 28, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "ORANGE", teamB: "VIOLET", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:00", duration: 40 * time.Minute},
				{teamA: "ORANGE", teamB: "VIOLET", sportType: constant.BASKETBALL_MALE_JR, startTime: "17:40", duration: 60 * time.Minute},
				{teamA: "ORANGE", teamB: "VIOLET", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:40", duration: 60 * time.Minute},
				{teamA: "PINK", teamB: "VIOLET", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 60 * time.Minute},
				{teamA: "PINK", teamB: "VIOLET", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 60 * time.Minute},
			},
		},
		{
			// 29 October 2025
			Date: time.Date(2025, 10, 29, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "GREEN", teamB: "YELLOW", sportType: constant.BASKETBALL_FEMALE_ALL, startTime: "17:00", duration: 40 * time.Minute},
				{teamA: "GREEN", teamB: "YELLOW", sportType: constant.BASKETBALL_MALE_JR, startTime: "17:40", duration: 60 * time.Minute},
				{teamA: "GREEN", teamB: "YELLOW", sportType: constant.BASKETBALL_MALE_SR, startTime: "18:40", duration: 60 * time.Minute},
				{teamA: "YELLOW", teamB: "GREEN", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 60 * time.Minute},
				{teamA: "YELLOW", teamB: "GREEN", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 60 * time.Minute},
				{teamA: "ORANGE", teamB: "VIOLET", sportType: constant.VOLLEYBALL_FEMALE_ALL, startTime: "17:00", duration: 40 * time.Minute},
			},
		},
		{
			// 30 October 2025
			Date: time.Date(2025, 10, 30, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "ORANGE", teamB: "VIOLET", sportType: constant.FOOTBALL_MALE_JR, startTime: "17:00", duration: 60 * time.Minute},
				{teamA: "ORANGE", teamB: "VIOLET", sportType: constant.FOOTBALL_MALE_SR, startTime: "18:00", duration: 60 * time.Minute},
				{teamA: "YELLOW", teamB: "VIOLET", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "17:40", duration: 50 * time.Minute},
			},
		},
		{
			// 31 October 2025
			Date: time.Date(2025, 10, 31, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "BLUE", teamB: "PINK", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "17:40", duration: 50 * time.Minute},
			},
		},
		{
			// 3 November 2025
			Date: time.Date(2025, 11, 3, 0, 0, 0, 0, time.UTC),
			Matches: []matchInfo{
				{teamA: "ORANGE", teamB: "VIOLET", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "16:40", duration: 40 * time.Minute},
				{teamA: "GREEN", teamB: "PINK", sportType: constant.VOLLEYBALL_MALE_ALL, startTime: "17:30", duration: 50 * time.Minute},
			},
		},
	}

	// Generate matches
	// Convert Bangkok times to UTC by subtracting 7 hours
	var matchesMock []model.Match
	for _, day := range matchSchedule {
		for _, m := range day.Matches {
			startTime, _ := time.Parse("15:04", m.startTime)
			// Create time in UTC, then subtract 7 hours to get the UTC equivalent of Bangkok time
			matchDateTime := time.Date(day.Date.Year(), day.Date.Month(), day.Date.Day(),
				startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)
			// Subtract 7 hours to convert Bangkok time to UTC
			matchDateTimeUTC := matchDateTime.Add(-7 * time.Hour)

			teamA := colorTeams[m.teamA].Id
			teamB := colorTeams[m.teamB].Id
			match := model.Match{
				Id:        uuid.NewString(),
				TeamA_Id:  &teamA,
				TeamB_Id:  &teamB,
				TypeId:    m.sportType,
				StartTime: matchDateTimeUTC,
				EndTime:   matchDateTimeUTC.Add(m.duration),
			}
			matchesMock = append(matchesMock, match)
		}
	}

	groupA := []model.Color{violet, blue, yellow}
	groupB := []model.Color{green, pink, orange}

	groupStages := []model.GroupStage{}
	// loop for group A
	for _, sport := range sportTypes {
		for _, color := range groupA {
			group := model.GroupStage{
				Id:      "A",
				TypeId:  sport.Id,
				ColorId: color.Id,
			}
			groupStages = append(groupStages, group)
		}

		for _, color := range groupB {
			group := model.GroupStage{
				Id:      "B",
				TypeId:  sport.Id,
				ColorId: color.Id,
			}
			groupStages = append(groupStages, group)
		}
	}

	// Create or update roles (skip if already exists)
	for _, role := range roles {
		if err := db.Where(model.Role{ID: role.ID}).FirstOrCreate(&role).Error; err != nil {
			log.Printf("Warning: Error upserting role %s: %v", role.ID, err)
		}
	}

	// Upsert colors and groups (update color assignment if exists, create if not)
	for _, color := range colors {
		// First, upsert the color itself
		if err := db.Where(model.Color{Id: color.Id}).Assign(model.Color{Title: color.Title}).FirstOrCreate(&color).Error; err != nil {
			log.Printf("Warning: Error upserting color %s: %v", color.Id, err)
		}

		// Then upsert each group member
		for _, group := range color.Members {
			if err := db.Where(model.IntaniaGroup{Id: group.Id}).Assign(model.IntaniaGroup{ColorId: color.Id}).FirstOrCreate(&group).Error; err != nil {
				log.Printf("Warning: Error upserting group %s: %v", group.Id, err)
			}
		}
	}

	if err := db.Create(&sportTypes).Error; err != nil {
		log.Printf("Error creating sport_types: %v", err)
	}
	if err := db.Create(&matchesMock).Error; err != nil {
		log.Printf("Error creating matches: %v", err)
	}
	if err := db.Create(&groupStages).Error; err != nil {
		log.Printf("Error creating group stages: %v", err)
	}

	log.Println("migration successful.")
}
