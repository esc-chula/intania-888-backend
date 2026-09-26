package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/esc-chula/intania-888-backend/internal/domain/policy"
	"github.com/esc-chula/intania-888-backend/internal/model"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/database"
	"github.com/esc-chula/intania-888-backend/utils/constant"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// seed installs mutable catalogue data without deleting or replacing existing rows.
// Historical match schedules are intentionally excluded.
func main() {
	policyFile := flag.String("policy-file", "", "import allowlist/blacklist entries from an external JSON file")
	policyDryRun := flag.Bool("policy-dry-run", false, "validate a policy import without writing it")
	flag.Parse()

	if *policyFile != "" || *policyDryRun {
		if *policyFile == "" {
			log.Fatal("--policy-dry-run requires --policy-file")
		}
		if err := importPolicies(*policyFile, *policyDryRun); err != nil {
			log.Fatal(err)
		}
		return
	}

	db := database.NewGormDatabase(config.GetConfig())
	err := db.Transaction(func(tx *gorm.DB) error {
		colors := []model.Color{
			{Id: "VIOLET", Title: "สีม่วง"},
			{Id: "BLUE", Title: "สีฟ้า"},
			{Id: "GREEN", Title: "สีเขียว"},
			{Id: "PINK", Title: "สีชมพู"},
			{Id: "ORANGE", Title: "สีส้ม"},
			{Id: "YELLOW", Title: "สีเหลือง"},
		}

		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&colors).Error; err != nil {
			return err
		}

		groupColors := map[string]string{
			"DOG": "VIOLET", "J": "VIOLET", "R": "VIOLET",
			"E": "BLUE", "K": "BLUE", "N": "BLUE",
			"B": "GREEN", "C": "GREEN", "M": "GREEN",
			"G": "PINK", "H": "PINK", "T": "PINK",
			"P": "ORANGE", "Q": "ORANGE", "S": "ORANGE",
			"A": "YELLOW", "F": "YELLOW", "L": "YELLOW",
		}

		groups := make([]model.IntaniaGroup, 0, len(groupColors))
		for id, color := range groupColors {
			groups = append(groups, model.IntaniaGroup{Id: id, ColorId: color})
		}

		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&groups).Error; err != nil {
			return err
		}

		sports := []model.SportType{
			{Id: constant.FOOTBALL_MALE_JR, Title: "ฟุตบอลชาย ปี 1"},
			{Id: constant.FOOTBALL_MALE_SR, Title: "ฟุตบอลชาย ปี 2-4"},
			{Id: constant.BASKETBALL_MALE_JR, Title: "บาสเก็ตบอลชาย ปี 1"},
			{Id: constant.BASKETBALL_MALE_SR, Title: "บาสเก็ตบอลชาย ปี 2-4"},
			{Id: constant.BASKETBALL_FEMALE_ALL, Title: "บาสเก็ตบอลหญิง รวมชั้นปี"},
			{Id: constant.VOLLEYBALL_MALE_ALL, Title: "วอลเลย์บอลชาย รวมชั้นปี"},
			{Id: constant.VOLLEYBALL_FEMALE_ALL, Title: "วอลเลย์บอลหญิง รวมชั้นปี"},
			{Id: constant.CHAIRBALL_FEMALE_ALL, Title: "แชร์บอลหญิง รวมชั้นปี"},
			{Id: constant.RUNNING, Title: "วิ่งเปี้ยว"},
			{Id: constant.TUG_OF_WAR, Title: "ชักเย่อ"},
			{Id: constant.TRADITIONAL_SPORTS, Title: "กีฬาพื้นบ้าน"},
			{Id: constant.TUG_OF_WAR_CHAK_YOR, Title: "ชักเย่อ"},
			{Id: constant.RUNNING_PIAW, Title: "วิ่งเปี้ยว"},
		}

		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&sports).Error
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Print("catalogue seed complete; no match schedule was imported")
}

func importPolicies(filename string, dryRun bool) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open policy import: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			log.Printf("close policy import: %v", closeErr)
		}
	}()

	var input policy.BootstrapFile
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return fmt.Errorf("decode policy import: %w", err)
	}
	if len(input.Entries) == 0 {
		return fmt.Errorf("policy import contains no entries")
	}
	if dryRun {
		for index, entry := range input.Entries {
			if _, err := policy.ValidateCreateInput(entry); err != nil {
				return fmt.Errorf("policy import entry %d: %w", index+1, err)
			}
		}
		log.Printf("validated %d policy entries", len(input.Entries))
		return nil
	}

	cfg := config.GetConfig()
	db := database.NewGormDatabase(cfg)
	policyCache := cache.NewRedisClient(cfg)
	service := policy.NewService(policy.NewRepository(db), policyCache, zap.NewNop())
	for index, entry := range input.Entries {
		if _, err := service.Create(entry); err != nil {
			return fmt.Errorf("policy import entry %d: %w", index+1, err)
		}
	}
	log.Printf("imported %d policy entries", len(input.Entries))
	return nil
}
