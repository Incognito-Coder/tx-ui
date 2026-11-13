package cmd

import (
	"fmt"
	"log"
	"os"

	"x-ui/config"
	"x-ui/internal/database"
	"x-ui/internal/web/service"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate [sqlite-to-pg | pg-to-sqlite]",
	Short: "Migrate database schema or transfer data between engines",
	Run: func(cmd *cobra.Command, args []string) {
		_ = godotenv.Load("/etc/x-ui/x-ui.env", "/usr/local/x-ui/.env", ".env")
		if len(args) > 0 {
			switch args[0] {
			case "sqlite-to-pg", "to-pg", "to-postgres":
				MigrateSQLiteToPostgres()
				return
			case "pg-to-sqlite", "to-sqlite":
				MigratePostgresToSQLite()
				return
			}
		}
		MigrateDb()
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}

func MigrateDb() {
	_ = godotenv.Load("/etc/x-ui/x-ui.env", "/usr/local/x-ui/.env", ".env")
	inboundService := service.InboundService{}

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Start migrating database...")
	inboundService.MigrateDB()
	fmt.Println("Migration done!")
}

func MigrateSQLiteToPostgres() {
	_ = godotenv.Load("/etc/x-ui/x-ui.env", "/usr/local/x-ui/.env", ".env")
	if config.GetDBType() != config.DBTypePostgres {
		log.Fatal("Current database configuration in /etc/x-ui/x-ui.env is not PostgreSQL.")
	}

	sqlitePath := config.GetDBPath()
	if _, err := os.Stat(sqlitePath); err != nil {
		log.Fatalf("SQLite database file not found at %s: %v", sqlitePath, err)
	}

	fmt.Println("Connecting to PostgreSQL...")
	err := database.InitDB("")
	if err != nil {
		log.Fatalf("Failed to initialize PostgreSQL connection: %v", err)
	}

	fmt.Printf("Migrating all data from SQLite (%s) to PostgreSQL...\n", sqlitePath)
	err = database.ImportSQLiteToPostgres(sqlitePath)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	inboundService := service.InboundService{}
	inboundService.MigrateDB()
	fmt.Println("Data migration from SQLite to PostgreSQL completed successfully!")
}

func MigratePostgresToSQLite() {
	_ = godotenv.Load("/etc/x-ui/x-ui.env", "/usr/local/x-ui/.env", ".env")
	if config.GetDBType() != config.DBTypePostgres {
		log.Fatal("PostgreSQL is not configured in /etc/x-ui/x-ui.env. Cannot export from PostgreSQL.")
	}

	fmt.Println("Connecting to PostgreSQL...")
	err := database.InitDB("")
	if err != nil {
		log.Fatalf("Failed to initialize PostgreSQL connection: %v", err)
	}

	sqlitePath := config.GetDBPath()
	fmt.Printf("Exporting all data from PostgreSQL to SQLite (%s)...\n", sqlitePath)
	err = database.ExportPostgresToSQLite(sqlitePath)
	if err != nil {
		log.Fatalf("Export to SQLite failed: %v", err)
	}

	_ = database.CloseDB()
	fmt.Println("Data export from PostgreSQL to SQLite completed successfully!")
}
