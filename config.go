package deviceauth

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
	"github.com/zalando/go-keyring"
)

var (
	storage keyStorage
	config  struct {
		ServiceURL string
		AppID      string
		KeysDir    string
	}
)

func Init(isEnvLoad bool) bool {
	if !isEnvLoad {
		if err := godotenv.Load(); err != nil {
			return false
		}
	}
	config.ServiceURL = getEnv("DEVICEAUTH_SERVICE_URL", "")
	config.AppID = getEnv("DEVICEAUTH_APP_ID", "default_app")
	config.KeysDir = getEnv("DEVICEAUTH_KEYS_DIR", "./.deviceauth")

	ks := newKeyringStorage("deviceauth", "default")
	if _, err := keyring.Get("deviceauth", "test"); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		fs, err := newFileStorage(config.KeysDir)
		if err != nil {
			return false
		}
		storage = fs
	} else {
		storage = ks
	}
	return true
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}
