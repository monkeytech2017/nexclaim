// Package config โหลด environment variables
package config

import (
	"fmt"
	"os"
)

// Config เก็บ configuration ทั้งหมดของ NexClaim
type Config struct {
	Port             string
	Env              string
	LogLevel         string
	DBHost           string
	DBPort           string
	DBName           string
	DBUser           string
	DBPass           string
	HISDBHost        string
	HISDBPort        string
	HISDBName        string
	HISDBUser        string
	HISDBPass        string
	FDHBaseURL       string
	FDHUsername      string
	FDHPassword      string
	FDHHCode         string
	CHIBaseURL       string
	CHIUsername      string
	CHIPassword      string
	HospitalName     string
	HospitalHCode    string
	HospitalChangwat string
}

// Load โหลด config จาก environment variables
// หมายเหตุ: ใช้ godotenv.Load() ก่อนเรียก Load() เพื่อโหลด .env file
func Load() (*Config, error) {
	c := &Config{
		Port:             getEnv("PORT", "8080"),
		Env:              getEnv("ENV", "development"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		DBHost:           getEnv("DB_HOST", "localhost"),
		DBPort:           getEnv("DB_PORT", "5432"),
		DBName:           getEnv("DB_NAME", "nexclaim"),
		DBUser:           os.Getenv("DB_USER"),
		DBPass:           os.Getenv("DB_PASS"),
		HISDBHost:        os.Getenv("HIS_DB_HOST"),
		HISDBPort:        getEnv("HIS_DB_PORT", "3306"),
		HISDBName:        os.Getenv("HIS_DB_NAME"),
		HISDBUser:        os.Getenv("HIS_DB_USER"),
		HISDBPass:        os.Getenv("HIS_DB_PASS"),
		FDHBaseURL:       getEnv("FDH_BASE_URL", "https://fdh.moph.go.th"),
		FDHUsername:      os.Getenv("FDH_USERNAME"),
		FDHPassword:      os.Getenv("FDH_PASSWORD"),
		FDHHCode:         os.Getenv("FDH_HCODE"),
		CHIBaseURL:       getEnv("CHI_BASE_URL", "https://cs8.chi.or.th"),
		CHIUsername:      os.Getenv("CHI_USERNAME"),
		CHIPassword:      os.Getenv("CHI_PASSWORD"),
		HospitalName:     os.Getenv("HOSPITAL_NAME"),
		HospitalHCode:    os.Getenv("HOSPITAL_HCODE"),
		HospitalChangwat: os.Getenv("HOSPITAL_CHANGWAT"),
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {
	if c.DBUser == "" {
		return fmt.Errorf("missing required env: DB_USER")
	}
	return nil
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBName, c.DBUser, c.DBPass)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
