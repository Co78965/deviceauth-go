package deviceauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

type keyStorage interface {
	LoadKeys(fingerprint string) (*keyPair, error)
	SaveKeys(fingerprint string, kp *keyPair) error
}

type keyPair struct {
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

func generateKeyPair() (*keyPair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &keyPair{
		PublicKey:  hex.EncodeToString(publicKey),
		PrivateKey: hex.EncodeToString(privateKey),
	}, nil
}

type keyringStorage struct {
	service string
	user    string
}

func newKeyringStorage(service, user string) *keyringStorage {
	return &keyringStorage{service: service, user: user}
}

func (ks *keyringStorage) LoadKeys(fingerprint string) (*keyPair, error) {
	keyName := ks.user + "_keys_" + fingerprint
	data, err := keyring.Get(ks.service, keyName)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			kp, err := generateKeyPair()
			if err != nil {
				return nil, err
			}
			if err := ks.SaveKeys(fingerprint, kp); err != nil {
				return nil, err
			}
			return kp, nil
		}
		return nil, err
	}
	var kp keyPair
	if err := json.Unmarshal([]byte(data), &kp); err != nil {
		return nil, err
	}
	return &kp, nil
}

func (ks *keyringStorage) SaveKeys(fingerprint string, kp *keyPair) error {
	keyName := ks.user + "_keys_" + fingerprint
	data, err := json.Marshal(kp)
	if err != nil {
		return err
	}
	return keyring.Set(ks.service, keyName, string(data))
}

type fileStorage struct {
	dir string
}

func newFileStorage(dir string) (*fileStorage, error) {
	if dir == "" {
		dir = ".deviceauth"
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	return &fileStorage{dir: dir}, nil
}

func (fs *fileStorage) LoadKeys(fingerprint string) (*keyPair, error) {
	path := filepath.Join(fs.dir, "keys_"+fingerprint+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			kp, err := generateKeyPair()
			if err != nil {
				return nil, err
			}
			if err := fs.SaveKeys(fingerprint, kp); err != nil {
				return nil, err
			}
			return kp, nil
		}
		return nil, err
	}
	var kp keyPair
	if err := json.Unmarshal(data, &kp); err != nil {
		return nil, err
	}
	return &kp, nil
}

func (fs *fileStorage) SaveKeys(fingerprint string, kp *keyPair) error {
	path := filepath.Join(fs.dir, "keys_"+fingerprint+".json")
	data, err := json.Marshal(kp)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
