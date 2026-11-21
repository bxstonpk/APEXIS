// Package config โหลดค่าจาก environment (และไฟล์ .env ถ้ามี)
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"apexis/backend/internal/platform/database"
)

type Config struct {
	HTTPAddr  string
	Database  database.Config
	JWTSecret string
	JWTTTL    time.Duration
	JWTIssuer string
	// InitialAdmin คือผู้ใช้ที่สร้างตอนเริ่มโปรแกรมถ้ายังไม่มี (ว่างทั้งคู่ = ไม่ seed)
	InitialAdmin InitialAdmin
}

type InitialAdmin struct {
	Email    string // INITIAL_ADMIN_USERNAME — ระบบเข้าสู่ระบบด้วยอีเมล จึงต้องเป็นอีเมล
	Password string // INITIAL_ADMIN_PASSWORD — 8-72 ตัวอักษร
}

func Load() (Config, error) {
	ttl, err := time.ParseDuration(env("JWT_TTL", "8h"))
	if err != nil {
		return Config{}, fmt.Errorf("config: JWT_TTL: %w", err)
	}
	cfg := Config{
		HTTPAddr: env("HTTP_ADDR", ":8080"),
		Database: database.Config{
			Connection: env("DATABASE_CONNECTION", "postgres"),
			Host:       env("DATABASE_HOST", "localhost"),
			Port:       env("DATABASE_PORT", "5432"),
			User:       env("DATABASE_USER", ""),
			Password:   env("DATABASE_PASSWORD", ""),
			Name:       env("DATABASE_NAME", ""),
			SSLMode:    env("DATABASE_SSLMODE", "disable"),
		},
		JWTSecret: env("JWT_SECRET", ""),
		JWTTTL:    ttl,
		JWTIssuer: env("JWT_ISSUER", "erp-api"),
		InitialAdmin: InitialAdmin{
			Email:    strings.TrimSpace(env("INITIAL_ADMIN_USERNAME", "")),
			Password: env("INITIAL_ADMIN_PASSWORD", ""),
		},
	}
	if (cfg.InitialAdmin.Email == "") != (cfg.InitialAdmin.Password == "") {
		return Config{}, errors.New("config: INITIAL_ADMIN_USERNAME and INITIAL_ADMIN_PASSWORD must be set together")
	}
	if cfg.Database.User == "" || cfg.Database.Name == "" {
		return Config{}, errors.New("config: DATABASE_USER and DATABASE_NAME are required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("config: JWT_SECRET must be at least 32 characters")
	}
	return cfg, nil
}

// LoadDotEnv อ่านไฟล์ KEY=VALUE แล้ว set เป็น env (ไม่ทับค่าที่ตั้งไว้แล้ว)
// ไฟล์ไม่มีอยู่ไม่ถือเป็น error
func LoadDotEnv(paths ...string) error {
	for _, p := range paths {
		f, err := os.Open(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			k, v, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			k = strings.TrimSpace(strings.TrimPrefix(k, "export "))
			v = strings.Trim(strings.TrimSpace(v), `"'`)
			if _, exists := os.LookupEnv(k); !exists {
				_ = os.Setenv(k, v)
			}
		}
		f.Close()
		if err := sc.Err(); err != nil {
			return err
		}
	}
	return nil
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
