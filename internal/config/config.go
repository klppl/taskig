package config

import (
	"fmt"
	"os"
)

type Config struct {
	GoogleClientID     string
	GoogleClientSecret string
	AllowedEmail       string
	SessionSecret      string
	BaseURL            string
	Port               string
	DBPath             string
}

func Load() (*Config, error) {
	c := &Config{
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		AllowedEmail:       os.Getenv("ALLOWED_EMAIL"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		BaseURL:            os.Getenv("BASE_URL"),
		Port:               os.Getenv("PORT"),
		DBPath:             os.Getenv("DB_PATH"),
	}
	if c.Port == "" {
		c.Port = "8080"
	}
	if c.DBPath == "" {
		c.DBPath = "data/google-tasks.db"
	}
	if c.GoogleClientID == "" || c.GoogleClientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET are required")
	}
	if c.AllowedEmail == "" {
		return nil, fmt.Errorf("ALLOWED_EMAIL is required")
	}
	if len(c.SessionSecret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 32 bytes")
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("BASE_URL is required")
	}
	return c, nil
}
