package config

import (
	"fmt"
	"os"
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

	DatabaseURL string
	HTTPAddr    string

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
		DatabaseURL:       required("DATABASE_URL"),
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
	if cfg.PageSize <= 0 || cfg.HTTPTimeout <= 0 || cfg.MaxRetries < 0 || cfg.RequestDelay < 0 {
		return nil, fmt.Errorf("PAGE_SIZE, HTTP_TIMEOUT_SEC musbat, MAX_RETRIES va REQUEST_DELAY_MS manfiy bo'lmasligi kerak")
	}

	return cfg, nil
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
