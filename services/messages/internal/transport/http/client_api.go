package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"messages/pkg/msgclient"
)

type clientInitRequest struct {
	KeysURL     string `json:"keysUrl"`
	MessagesURL string `json:"messagesUrl"`
	UserID      string `json:"userId"`
	DeviceID    string `json:"deviceId"`
}

type clientInitResponse struct {
	State          string `json:"state"`
	UserID         string `json:"userId"`
	DeviceID       string `json:"deviceId"`
	KeysURL        string `json:"keysUrl"`
	MessagesURL    string `json:"messagesUrl"`
	OneTimePrekeys int    `json:"oneTimePrekeys"`
}

type clientSendRequest struct {
	State      string `json:"state"`
	ConvID     string `json:"convId"`
	ToDeviceID string `json:"toDeviceId"`
	Plaintext  string `json:"plaintext"`
}

type clientSendResponse struct {
	State string `json:"state"`
}

type clientEnvelopeRequest struct {
	State    string          `json:"state"`
	Envelope json.RawMessage `json:"envelope"`
}

type clientEnvelopeResponse struct {
	State     string `json:"state"`
	Plaintext string `json:"plaintext"`
}

const clientHTTPTimeout = 10 * time.Second

func (h *Handler) handleClientInit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req clientInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	userID := strings.TrimSpace(req.UserID)
	deviceIDParam := strings.TrimSpace(req.DeviceID)
	if deviceIDParam == "" {
		http.Error(w, "deviceId is required", http.StatusBadRequest)
		return
	}
	var deviceUUID uuid.UUID
	if deviceIDParam != "" {
		var err error
		deviceUUID, err = uuid.Parse(deviceIDParam)
		if err != nil {
			http.Error(w, "invalid deviceId", http.StatusBadRequest)
			return
		}
		deviceIDParam = deviceUUID.String()
	}
	claims, ok := h.requireAuth(w, r, deviceUUID)
	if !ok {
		return
	}
	if userID != "" && userID != claims.UserID.String() {
		http.Error(w, "userId does not match token subject", http.StatusForbidden)
		return
	}
	if userID == "" {
		userID = claims.UserID.String()
	}
	opts := msgclient.InitOptions{
		KeysBaseURL:     strings.TrimSpace(req.KeysURL),
		MessagesBaseURL: strings.TrimSpace(req.MessagesURL),
		UserID:          userID,
		DeviceID:        deviceIDParam,
		AccessToken:     extractToken(r),
	}
	if opts.KeysBaseURL == "" || opts.MessagesBaseURL == "" {
		http.Error(w, "keysUrl and messagesUrl are required", http.StatusBadRequest)
		return
	}
	state, reg, err := msgclient.RegisterDevice(r.Context(), opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	data, err := state.Marshal()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := clientInitResponse{
		State:          string(data),
		UserID:         reg.UserID,
		DeviceID:       reg.DeviceID,
		KeysURL:        state.KeysBaseURL(),
		MessagesURL:    state.MessagesBaseURL(),
		OneTimePrekeys: reg.OneTimePreKeys,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleClientSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req clientSendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	stateData := strings.TrimSpace(req.State)
	if stateData == "" {
		http.Error(w, "state is required", http.StatusBadRequest)
		return
	}
	convID, err := uuid.Parse(strings.TrimSpace(req.ConvID))
	if err != nil {
		http.Error(w, "invalid convId", http.StatusBadRequest)
		return
	}
	toID, err := uuid.Parse(strings.TrimSpace(req.ToDeviceID))
	if err != nil {
		http.Error(w, "invalid toDeviceId", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Plaintext) == "" {
		http.Error(w, "plaintext is required", http.StatusBadRequest)
		return
	}
	if _, ok := h.requireAuth(w, r, uuid.Nil); !ok {
		return
	}
	state, err := msgclient.LoadStateFromJSON([]byte(stateData))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	prepared, err := state.PrepareSend(convID, toID, req.Plaintext)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := postEncryptedMessage(r.Context(), extractToken(r), state.MessagesBaseURL(), prepared); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	data, err := state.Marshal()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, clientSendResponse{State: string(data)})
}

func (h *Handler) handleClientEnvelope(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req clientEnvelopeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if _, ok := h.requireAuth(w, r, uuid.Nil); !ok {
		return
	}
	stateData := strings.TrimSpace(req.State)
	if stateData == "" {
		http.Error(w, "state is required", http.StatusBadRequest)
		return
	}
	var env msgclient.InboundEnvelope
	if len(req.Envelope) == 0 {
		http.Error(w, "envelope is required", http.StatusBadRequest)
		return
	}
	if err := json.Unmarshal(req.Envelope, &env); err != nil {
		http.Error(w, "invalid envelope", http.StatusBadRequest)
		return
	}
	state, err := msgclient.LoadStateFromJSON([]byte(stateData))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	plaintext, err := state.HandleEnvelope(&env)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data, err := state.Marshal()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := clientEnvelopeResponse{State: string(data), Plaintext: plaintext}
	writeJSON(w, http.StatusOK, resp)
}

func postEncryptedMessage(ctx context.Context, token string, base string, payload any) error {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return errors.New("message base URL missing")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint := base + "/messages/send"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token = strings.TrimSpace(token); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: clientHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= http.StatusBadRequest {
		data, _ := io.ReadAll(resp.Body)
		if len(data) == 0 {
			data = []byte(resp.Status)
		}
		return fmt.Errorf("send failed: %s", strings.TrimSpace(string(data)))
	}
	return nil
}
