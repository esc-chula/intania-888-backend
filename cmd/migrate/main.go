package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/esc-chula/intania-888-backend/pkg/config"
	"github.com/esc-chula/intania-888-backend/pkg/database"
	"github.com/pressly/goose/v3"
)

func main() {
	flag.Parse()

	command := "status"
	if flag.NArg() > 0 {
		command = flag.Arg(0)
	}

	if command == "down" || command == "reset" {
		if os.Getenv("ALLOW_DESTRUCTIVE_MIGRATIONS") != "I_UNDERSTAND_DATA_WILL_BE_LOST" {
			log.Fatal("destructive migration refused: set ALLOW_DESTRUCTIVE_MIGRATIONS=I_UNDERSTAND_DATA_WILL_BE_LOST")
		}
	}

	db := database.NewGormDatabase(config.GetConfig())
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}

	defer sqlDB.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal(err)
	}

	switch command {
	case "up":
		err = goose.Up(sqlDB, "migrations")
	case "status":
		err = goose.Status(sqlDB, "migrations")
	case "down":
		err = goose.Down(sqlDB, "migrations")
	case "reset":
		err = goose.Reset(sqlDB, "migrations")
	default:
		err = fmt.Errorf("unknown migration command %q", command)
	}

	if err != nil {
		log.Fatal(err)
	}
}
