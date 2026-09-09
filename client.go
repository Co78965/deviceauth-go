package deviceauth

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type client struct {
	baseURL    string
	httpClient *http.Client
}

func newClient() *client {
	return &client{
		baseURL: config.ServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *client) request(path string, payload interface{}) (int, []byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, err
	}

	resp, err := c.httpClient.Post(c.baseURL+path, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func (c *client) registerDevice(userID, fingerprint, publicKey string) (deviceID, challenge string, err error) {
	payload := map[string]string{
		"user_id":     userID,
		"app_id":      config.AppID,
		"fingerprint": fingerprint,
		"public_key":  publicKey,
	}

	status, body, err := c.request("/api/devices/register", payload)
	if err != nil {
		return "", "", err
	}

	if status != http.StatusCreated {
		return "", "", parseServiceError(body)
	}

	var resp struct {
		DeviceID  string `json:"device_id"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", err
	}
	return resp.DeviceID, resp.Challenge, nil
}

func (c *client) getChallenge(fingerprint, userID string) (deviceID, challenge string, err error) {
	payload := map[string]string{
		"app_id":      config.AppID,
		"fingerprint": fingerprint,
	}
	if userID != "" {
		payload["user_id"] = userID
	}

	status, body, err := c.request("/api/auth/challenge", payload)
	if err != nil {
		return "", "", err
	}

	if status != http.StatusOK {
		return "", "", parseServiceError(body)
	}

	var resp struct {
		DeviceID  string `json:"device_id"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", "", err
	}
	return resp.DeviceID, resp.Challenge, nil
}

func (c *client) verifySignature(deviceID, signature, flow string) (userID string, err error) {
	payload := map[string]string{
		"device_id": deviceID,
		"signature": signature,
		"flow":      flow,
	}

	status, body, err := c.request("/api/auth/verify", payload)
	if err != nil {
		return "", err
	}

	if status != http.StatusOK {
		return "", parseServiceError(body)
	}

	var resp struct {
		UserID string `json:"user_id,omitempty"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	return resp.UserID, nil
}

func parseServiceError(body []byte) error {
	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != "" {
		switch errResp.Error {
		case "multiple_devices":
			return &serviceError{Code: errMultipleDevices}
		default:
			return &serviceError{Code: errAuthFailed}
		}
	}
	return &serviceError{Code: errAuthFailed}
}
