package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cast"
)

type Config struct {
	// DONA stendi. Bu qiymatlar hech qachon logga chiqmaydi.
	DonaBaseURL       string
	DonaEmail         string
	DonaPassword      string
	DonaWarehouseCode string
	DonaWarehouseID   string

	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDatabase string

	HTTPAddr string

	PageSize     int
	RequestDelay time.Duration
	HTTPTimeout  time.Duration
	MaxRetries   int
	SyncOnStart  bool
}

// Load env'dan konfiguratsiyani o'qiydi. Majburiy kalit bo'lmasa, xato qaytaradi
// va ilova aniq xabar bilan to'xtaydi.
func Load() (*Config, error) {
	// .env ixtiyoriy: docker'da qiymatlar env_file orqali keladi.
	_ = godotenv.Load(".env")

	var missing []string
	required := func(key string) string {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	cfg := &Config{
		DonaBaseURL:       strings.TrimRight(required("DONA_API_BASE_URL"), "/"),
		DonaEmail:         required("DONA_EMAIL"),
		DonaPassword:      required("DONA_PASSWORD"),
		DonaWarehouseCode: cast.ToString(getOrReturnDefaultValue("DONA_WAREHOUSE_CODE", "")),
		DonaWarehouseID:   cast.ToString(getOrReturnDefaultValue("DONA_WAREHOUSE_ID", "")),
		PostgresHost:      cast.ToString(getOrReturnDefaultValue("POSTGRES_HOST", "localhost")),
		PostgresPort:      cast.ToInt(getOrReturnDefaultValue("POSTGRES_PORT", 5432)),
		PostgresUser:      cast.ToString(getOrReturnDefaultValue("POSTGRES_USER", "postgres")),
		PostgresPassword:  required("POSTGRES_PASSWORD"),
		PostgresDatabase:  cast.ToString(getOrReturnDefaultValue("POSTGRES_DATABASE", "order_monitoring")),
		HTTPAddr:          cast.ToString(getOrReturnDefaultValue("HTTP_ADDR", ":8080")),
		PageSize:          cast.ToInt(getOrReturnDefaultValue("PAGE_SIZE", 50)),
		RequestDelay:      time.Duration(cast.ToInt(getOrReturnDefaultValue("REQUEST_DELAY_MS", 300))) * time.Millisecond,
		HTTPTimeout:       time.Duration(cast.ToInt(getOrReturnDefaultValue("HTTP_TIMEOUT_SEC", 15))) * time.Second,
		MaxRetries:        cast.ToInt(getOrReturnDefaultValue("MAX_RETRIES", 5)),
		SyncOnStart:       cast.ToBool(getOrReturnDefaultValue("SYNC_ON_START", false)),
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("majburiy env kalit(lar) to'ldirilmagan: %s", strings.Join(missing, ", "))
	}
	if cfg.PostgresPort <= 0 {
		return nil, fmt.Errorf("POSTGRES_PORT musbat son bo'lishi kerak")
	}
	if cfg.PageSize <= 0 || cfg.HTTPTimeout <= 0 || cfg.MaxRetries < 0 || cfg.RequestDelay < 0 {
		return nil, fmt.Errorf("PAGE_SIZE, HTTP_TIMEOUT_SEC musbat, MAX_RETRIES va REQUEST_DELAY_MS manfiy bo'lmasligi kerak")
	}

	return cfg, nil
}

// PostgresURL alohida kalitlardan DSN yig'adi. url.URL parol ichidagi maxsus belgilarni
// (@, :, / ...) to'g'ri kodlaydi. Natija logga chiqarilmaydi.
func (c *Config) PostgresURL() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.PostgresUser, c.PostgresPassword),
		Host:     net.JoinHostPort(c.PostgresHost, strconv.Itoa(c.PostgresPort)),
		Path:     c.PostgresDatabase,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

// getOrReturnDefaultValue rider'dagidek, lekin bo'sh qiymatni ham "yo'q" deb hisoblaydi:
// .env.example nusxalanganda kalitlar bo'sh qoladi.
func getOrReturnDefaultValue(key string, defaultValue interface{}) interface{} {
	val, exists := os.LookupEnv(key)
	if exists && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}

	return defaultValue
}
