package deviceauth

import (
	"crypto/ed25519"
	"encoding/hex"
)

func (kp *keyPair) signChallenge(challenge string) (string, error) {
	privBytes, err := hex.DecodeString(kp.PrivateKey)
	if err != nil {
		return "", err
	}
	priv := ed25519.PrivateKey(privBytes)

	signature := ed25519.Sign(priv, []byte(challenge))
	return hex.EncodeToString(signature), nil
}
