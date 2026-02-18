package db

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func Init() {
	dsn := os.Getenv("MYSQL_DSN")

	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to DB:", err)
	}

	if err = DB.Ping(); err != nil {
		log.Fatal("DB unreachable:", err)
	}

	log.Println("Database connected")
}
