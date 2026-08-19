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
		databaseURL = "postgresql://postgres.akgivxaziyfyduidzvxk:SklGo2026Secure@aws-0-ap-south-1.pooler.supabase.com:5432/postgres"
	}

	var err error
	DB, err = pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal terhubung ke database: %v\n", err)
		os.Exit(1)
	}

	// Test koneksi
	err = DB.Ping(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database tidak merespons: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Berhasil terhubung ke database")
}