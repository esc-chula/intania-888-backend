package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/esc-chula/intania-888-backend/internal/domain/location"
	"github.com/esc-chula/intania-888-backend/internal/domain/policy"
	"github.com/esc-chula/intania-888-backend/internal/domain/sporttype"
	persistence "github.com/esc-chula/intania-888-backend/internal/persistence/model"
	"github.com/esc-chula/intania-888-backend/pkg/cache"
	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/database"
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

	db := database.NewGORMDatabase(config.GetConfig())
	if err := seedCatalogue(db); err != nil {
		log.Fatal(err)
	}

	log.Print("catalogue seed complete; no match schedule was imported")
}

// seedCatalogue inserts missing defaults, preserving edits to existing titles.
// Explicitly rerunning it restores deleted default sport IDs.
func seedCatalogue(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		colors := []persistence.Color{
			{
				ID:    "VIOLET",
				Title: "สีม่วง",
			},
			{
				ID:    "BLUE",
				Title: "สีฟ้า",
			},
			{
				ID:    "GREEN",
				Title: "สีเขียว",
			},
			{
				ID:    "PINK",
				Title: "สีชมพู",
			},
			{
				ID:    "ORANGE",
				Title: "สีส้ม",
			},
			{
				ID:    "YELLOW",
				Title: "สีเหลือง",
			},
		}

		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&colors).Error; err != nil {
			return err
		}

		groupColors := map[string]string{
			"DOG": "VIOLET",
			"J":   "VIOLET",
			"R":   "VIOLET",
			"E":   "BLUE",
			"K":   "BLUE",
			"N":   "BLUE",
			"B":   "GREEN",
			"C":   "GREEN",
			"M":   "GREEN",
			"G":   "PINK",
			"H":   "PINK",
			"T":   "PINK",
			"P":   "ORANGE",
			"Q":   "ORANGE",
			"S":   "ORANGE",
			"A":   "YELLOW",
			"F":   "YELLOW",
			"L":   "YELLOW",
		}

		groups := make([]persistence.IntaniaGroup, 0, len(groupColors))
		for id, color := range groupColors {
			groups = append(groups, persistence.IntaniaGroup{
				ID:      id,
				ColorID: color,
			})
		}

		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&groups).Error; err != nil {
			return err
		}

		sports := []persistence.SportType{
			{
				ID:    sporttype.FootballMaleJunior,
				Title: "ฟุตบอลชาย ปี 1",
			},
			{
				ID:    sporttype.FootballMaleSenior,
				Title: "ฟุตบอลชาย ปี 2-4",
			},
			{
				ID:    sporttype.BasketballMaleJunior,
				Title: "บาสเก็ตบอลชาย ปี 1",
			},
			{
				ID:    sporttype.BasketballMaleSenior,
				Title: "บาสเก็ตบอลชาย ปี 2-4",
			},
			{
				ID:    sporttype.BasketballFemaleAll,
				Title: "บาสเก็ตบอลหญิง รวมชั้นปี",
			},
			{
				ID:    sporttype.VolleyballMaleAll,
				Title: "วอลเลย์บอลชาย รวมชั้นปี",
			},
			{
				ID:    sporttype.VolleyballFemaleAll,
				Title: "วอลเลย์บอลหญิง รวมชั้นปี",
			},
			{
				ID:    sporttype.ChairballFemaleAll,
				Title: "แชร์บอลหญิง รวมชั้นปี",
			},
			{
				ID:    sporttype.Running,
				Title: "วิ่งเปี้ยว",
			},
			{
				ID:    sporttype.TugOfWar,
				Title: "ชักเย่อ",
			},
			{
				ID:    sporttype.TraditionalSports,
				Title: "กีฬาพื้นบ้าน",
			},
			{
				ID:    sporttype.TugOfWarChakYor,
				Title: "ชักเย่อ",
			},
			{
				ID:    sporttype.RunningPiaw,
				Title: "วิ่งเปี้ยว",
			},
		}
		locations := []persistence.Location{
			{
				ID:    location.CivilCourt,
				Title: "สนามโยธา",
			},
			{
				ID:    location.TwoReignsStatuePlaza,
				Title: "ลานพระบรมรูปสองรัชกาล",
			},
			{
				ID:    location.CentennialBuildingFloor,
				Title: "ตึก 100 ปี ชั้น 12",
			},
			{
				ID:    location.GearLawn,
				Title: "ลานเกียร์",
			},
			{
				ID:    location.IndoorStadiumOne,
				Title: "สนามกีฬาในร่ม 1",
			},
			{
				ID:    location.JubStadium,
				Title: "สนามจุ๊บ",
			},
		}

		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&sports).Error; err != nil {
			return err
		}

		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&locations).Error
	})
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
			if _, err := policy.ValidateCreateInput(entry.Input()); err != nil {
				return fmt.Errorf("policy import entry %d: %w", index+1, err)
			}
		}
		log.Printf("validated %d policy entries", len(input.Entries))

		return nil
	}

	cfg := config.GetConfig()
	db := database.NewGORMDatabase(cfg)
	policyCache := cache.NewRedisClient(cfg)
	service := policy.NewService(policy.NewGORMRepository(db), policy.NewRedisSnapshotCache(policyCache), zap.NewNop())
	for index, entry := range input.Entries {
		if _, err := service.Create(context.Background(), entry.Input()); err != nil {
			return fmt.Errorf("policy import entry %d: %w", index+1, err)
		}
	}
	log.Printf("imported %d policy entries", len(input.Entries))

	return nil
}
