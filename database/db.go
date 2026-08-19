package database

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func ConnectDB() {
	databaseURL := os.Getenv("DATABASE_URL")
	
	if databaseURL == "" {
		fmt.Fprintf(os.Stderr, "Fatal Error: Environment variable DATABASE_URL tidak ditemukan!\n")
		os.Exit(1)
	}

	var err error
	DB, err = pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal membuat pool database: %v\n", err)
		os.Exit(1)
	}

	err = DB.Ping(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database tidak merespons: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Berhasil terhubung ke database")
}