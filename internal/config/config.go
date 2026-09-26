package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port                string
	DatabaseURL         string
	SupabaseJWTSecret   string
	SupabaseJWTIssuer   string
	SupabaseJWTAudience string
	CORSAllowedOrigins  []string
	MapboxAccessToken   string
	AeroDataBoxAPIKey   string
	AeroDataBoxHost     string
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		SupabaseJWTSecret:   os.Getenv("SUPABASE_JWT_SECRET"),
		SupabaseJWTIssuer:   os.Getenv("SUPABASE_JWT_ISSUER"),
		SupabaseJWTAudience: os.Getenv("SUPABASE_JWT_AUDIENCE"),
		CORSAllowedOrigins:  getCSVEnv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:8081", "http://127.0.0.1:8081", "http://localhost:19006", "http://127.0.0.1:19006", "https://app.eleventravel.com"}),
		MapboxAccessToken:   os.Getenv("MAPBOX_ACCESS_TOKEN"),
		AeroDataBoxAPIKey:   os.Getenv("AERODATABOX_RAPIDAPI_KEY"),
		AeroDataBoxHost:     getEnv("AERODATABOX_RAPIDAPI_HOST", "aerodatabox.p.rapidapi.com"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	if cfg.SupabaseJWTSecret == "" && cfg.SupabaseJWTIssuer == "" {
		return Config{}, errors.New("SUPABASE_JWT_SECRET or SUPABASE_JWT_ISSUER is required")
	}

	return cfg, nil
}

func (c Config) HTTPAddress() string {
	return fmt.Sprintf(":%s", c.Port)
}

func getEnv(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getCSVEnv(key string, fallback []string) []string {
	rawValue := strings.TrimSpace(os.Getenv(key))
	if rawValue == "" {
		return fallback
	}

	parts := strings.Split(rawValue, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin != "" {
			origins = append(origins, origin)
		}
	}

	if len(origins) == 0 {
		return fallback
	}

	return origins
}

func loadDotEnv() error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return err
	}

	for {
		candidatePath := filepath.Join(workingDirectory, ".env")
		if loadErr := loadDotEnvFile(candidatePath); loadErr != nil {
			return loadErr
		}

		parentDirectory := filepath.Dir(workingDirectory)
		if parentDirectory == workingDirectory {
			return nil
		}

		workingDirectory = parentDirectory
	}
}

func loadDotEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"`)

		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
