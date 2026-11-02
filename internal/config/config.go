package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port                string
	DatabaseURL         string
	DatabaseMaxConns    int32
	SupabaseJWTSecret   string
	SupabaseJWTIssuer   string
	SupabaseJWTAudience string
	SupabaseURL         string
	SupabaseServiceRole string
	CORSAllowedOrigins  []string
	MapboxAccessToken   string
	AeroDataBoxAPIKey   string
	AeroDataBoxHost     string

	R2AccessKeyID     string
	R2SecretAccessKey string
	R2Endpoint        string
	R2Region          string
	R2PublicBucket    string
	R2PrivateBucket   string
	R2PublicDomain    string

	RegistrationEnabled            bool
	RegistrationAccessCodeRequired bool
	RegistrationAccessCode         string
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	registrationEnabled, err := getBoolEnv("REGISTRATION_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	registrationAccessCodeRequired, err := getBoolEnv("REGISTRATION_ACCESS_CODE_REQUIRED", false)
	if err != nil {
		return Config{}, err
	}

	r2Endpoint := strings.TrimSpace(os.Getenv("R2_ENDPOINT"))
	if accountID := strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID")); r2Endpoint == "" && accountID != "" {
		r2Endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	}

	cfg := Config{
		Port:                           getEnv("PORT", "8080"),
		DatabaseURL:                    os.Getenv("DATABASE_URL"),
		DatabaseMaxConns:               getEnvInt32("DATABASE_MAX_CONNS", 3),
		SupabaseJWTSecret:              os.Getenv("SUPABASE_JWT_SECRET"),
		SupabaseJWTIssuer:              os.Getenv("SUPABASE_JWT_ISSUER"),
		SupabaseJWTAudience:            os.Getenv("SUPABASE_JWT_AUDIENCE"),
		SupabaseURL:                    strings.TrimSpace(os.Getenv("SUPABASE_URL")),
		SupabaseServiceRole:            strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_ROLE_KEY")),
		CORSAllowedOrigins:             getCSVEnv("CORS_ALLOWED_ORIGINS", []string{"http://localhost:8081", "http://127.0.0.1:8081", "http://localhost:19006", "http://127.0.0.1:19006", "https://app.eleventravel.com"}),
		MapboxAccessToken:              os.Getenv("MAPBOX_ACCESS_TOKEN"),
		AeroDataBoxAPIKey:              os.Getenv("AERODATABOX_RAPIDAPI_KEY"),
		AeroDataBoxHost:                getEnv("AERODATABOX_RAPIDAPI_HOST", "aerodatabox.p.rapidapi.com"),
		R2AccessKeyID:                  strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID")),
		R2SecretAccessKey:              strings.TrimSpace(os.Getenv("R2_SECRET_ACCESS_KEY")),
		R2Endpoint:                     r2Endpoint,
		R2Region:                       getEnv("R2_REGION", "auto"),
		R2PublicBucket:                 strings.TrimSpace(os.Getenv("R2_PUBLIC_BUCKET")),
		R2PrivateBucket:                strings.TrimSpace(os.Getenv("R2_PRIVATE_BUCKET")),
		R2PublicDomain:                 strings.TrimRight(strings.TrimSpace(os.Getenv("R2_PUBLIC_DOMAIN")), "/"),
		RegistrationEnabled:            registrationEnabled,
		RegistrationAccessCodeRequired: registrationAccessCodeRequired,
		RegistrationAccessCode:         strings.TrimSpace(os.Getenv("REGISTRATION_ACCESS_CODE")),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.DatabaseMaxConns < 1 {
		return Config{}, errors.New("DATABASE_MAX_CONNS must be greater than zero")
	}

	if cfg.SupabaseJWTSecret == "" && cfg.SupabaseJWTIssuer == "" {
		return Config{}, errors.New("SUPABASE_JWT_SECRET or SUPABASE_JWT_ISSUER is required")
	}

	if cfg.RegistrationEnabled {
		if cfg.SupabaseURL == "" || cfg.SupabaseServiceRole == "" {
			return Config{}, errors.New("SUPABASE_URL and SUPABASE_SERVICE_ROLE_KEY are required when REGISTRATION_ENABLED=true")
		}
		if cfg.RegistrationAccessCodeRequired && cfg.RegistrationAccessCode == "" {
			return Config{}, errors.New("REGISTRATION_ACCESS_CODE is required when REGISTRATION_ACCESS_CODE_REQUIRED=true")
		}
	}

	return cfg, nil
}

// R2Configured reports whether every setting needed to talk to Cloudflare R2 is present.
func (c Config) R2Configured() bool {
	return c.R2AccessKeyID != "" && c.R2SecretAccessKey != "" && c.R2Endpoint != "" &&
		c.R2PublicBucket != "" && c.R2PrivateBucket != "" && c.R2PublicDomain != ""
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

func getEnvInt32(key string, fallback int32) int32 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return fallback
	}

	return int32(parsed)
}

func getBoolEnv(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}

	return parsed, nil
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
