package deviceauth

import (
	"errors"
	"os"

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

func init() {
	config.ServiceURL = getEnv("DEVICEAUTH_SERVICE_URL", "")
	config.AppID = getEnv("DEVICEAUTH_APP_ID", "default_app")
	config.KeysDir = getEnv("DEVICEAUTH_KEYS_DIR", "./.deviceauth")

	ks := newKeyringStorage("deviceauth", "default")
	if _, err := keyring.Get("deviceauth", "test"); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		fs, err := newFileStorage(config.KeysDir)
		if err != nil {
			panic(err)
		}
		storage = fs
	} else {
		storage = ks
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}
