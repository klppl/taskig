package database

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath, filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='sessions'").Scan(&tableName)
	if err != nil {
		t.Fatalf("sessions table not found: %v", err)
	}
	if tableName != "sessions" {
		t.Fatalf("expected 'sessions', got %q", tableName)
	}
}

func TestOpenCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "test.db")

	db, err := Open(dbPath, filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if _, err := os.Stat(filepath.Join(dir, "subdir")); os.IsNotExist(err) {
		t.Fatal("expected subdir to be created")
	}
}
