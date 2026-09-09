package deviceauth

import (
	"encoding/json"
	"net/http"
)

type authRequest struct {
	Fingerprint string `json:"fingerprint"`
	UserID      string `json:"user_id,omitempty"`
}

type authResponse struct {
	Status  string `json:"status,omitempty"`
	UserID  string `json:"user_id,omitempty"`
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func RegisterHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, authResponse{Error: "method_not_allowed"})
			return
		}

		var req authRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, authResponse{Error: "invalid_request"})
			return
		}
		if req.Fingerprint == "" || req.UserID == "" {
			writeJSON(w, http.StatusBadRequest, authResponse{Error: "fingerprint_and_user_id_required"})
			return
		}

		resp, err := processRegister(req.Fingerprint, req.UserID)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func AuthenticateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, authResponse{Error: "method_not_allowed"})
			return
		}

		var req authRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, authResponse{Error: "invalid_request"})
			return
		}
		if req.Fingerprint == "" {
			writeJSON(w, http.StatusBadRequest, authResponse{Error: "fingerprint_required"})
			return
		}

		resp, err := processAuthenticate(req.Fingerprint, req.UserID)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func processRegister(fingerprint, userID string) (authResponse, error) {
	client := newClient()
	keys, err := storage.LoadKeys(fingerprint)
	if err != nil {
		return authResponse{}, err
	}

	deviceID, challenge, err := client.registerDevice(userID, fingerprint, keys.PublicKey)
	if err != nil {
		return authResponse{}, err
	}

	signature, err := keys.signChallenge(challenge)
	if err != nil {
		return authResponse{}, err
	}

	_, err = client.verifySignature(deviceID, signature, "register")
	if err != nil {
		return authResponse{}, err
	}

	return authResponse{Status: statusRegistered, UserID: userID}, nil
}

func processAuthenticate(fingerprint, userID string) (authResponse, error) {
	client := newClient()
	keys, err := storage.LoadKeys(fingerprint)
	if err != nil {
		return authResponse{}, err
	}

	deviceID, challenge, err := client.getChallenge(fingerprint, userID)
	if err != nil {
		return authResponse{}, err
	}

	signature, err := keys.signChallenge(challenge)
	if err != nil {
		return authResponse{}, err
	}

	authUserID, err := client.verifySignature(deviceID, signature, "auth")
	if err != nil {
		return authResponse{}, err
	}

	return authResponse{Status: statusAuthenticated, UserID: authUserID}, nil
}

func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusUnauthorized
	code := errAuthFailed
	if svcErr, ok := err.(*serviceError); ok {
		switch svcErr.Code {
		case errMultipleDevices:
			status = http.StatusConflict
			code = errMultipleDevices
		}
	}
	writeJSON(w, status, authResponse{Error: code})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
