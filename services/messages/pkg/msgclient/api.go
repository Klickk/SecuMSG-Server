package msgclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	cryptocore "cryptocore"
	"github.com/google/uuid"
)

// InitOptions configures device registration.
type InitOptions struct {
	KeysBaseURL     string
	MessagesBaseURL string
	UserID          string
	DeviceID        string
	AccessToken     string
}

// RegisterDevice provisions a new device with the key service and builds a runtime state.
func RegisterDevice(ctx context.Context, opts InitOptions) (*State, registerDeviceResponse, error) {
	dev, err := cryptocore.GenerateIdentityKeypair()
	if err != nil {
		return nil, registerDeviceResponse{}, fmt.Errorf("generate identity: %w", err)
	}
	bundle, err := dev.PublishPrekeyBundle(0)
	if err != nil {
		return nil, registerDeviceResponse{}, fmt.Errorf("publish bundle: %w", err)
	}
	req := registerDeviceRequest{
		UserID:               strings.TrimSpace(opts.UserID),
		DeviceID:             strings.TrimSpace(opts.DeviceID),
		IdentityKey:          base64.StdEncoding.EncodeToString(bundle.IdentityKey[:]),
		IdentitySignatureKey: base64.StdEncoding.EncodeToString(bundle.IdentitySignatureKey),
	}
	req.SignedPreKey.PublicKey = base64.StdEncoding.EncodeToString(bundle.SignedPrekey[:])
	req.SignedPreKey.Signature = base64.StdEncoding.EncodeToString(bundle.SignedPrekeySig)
	req.SignedPreKey.CreatedAt = time.Now().UTC()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, registerDeviceResponse{}, err
	}
	endpoint := joinURL(opts.KeysBaseURL, "/keys/device/register")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, registerDeviceResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(opts.AccessToken); token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, registerDeviceResponse{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		if len(data) == 0 {
			data = []byte(resp.Status)
		}
		return nil, registerDeviceResponse{}, fmt.Errorf("register request failed: %s", strings.TrimSpace(string(data)))
	}
	var regResp registerDeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return nil, registerDeviceResponse{}, err
	}
	state := &State{
		file: stateFile{
			UserID:          regResp.UserID,
			DeviceID:        regResp.DeviceID,
			KeysBaseURL:     normalizeBaseURL(opts.KeysBaseURL),
			MessagesBaseURL: normalizeBaseURL(opts.MessagesBaseURL),
		},
		device:   dev,
		sessions: make(map[string]*cryptocore.SessionState),
	}
	return state, regResp, nil
}

// LoadStateFromJSON reconstructs a State from its serialized JSON form.
func LoadStateFromJSON(data []byte) (*State, error) {
	var file stateFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if file.Device == nil {
		return nil, errors.New("state file missing device")
	}
	dev, err := cryptocore.ImportDevice(file.Device)
	if err != nil {
		return nil, err
	}
	sessions := make(map[string]*cryptocore.SessionState)
	for id, snap := range file.Sessions {
		sess, err := cryptocore.ImportSession(snap)
		if err != nil {
			return nil, fmt.Errorf("import session %s: %w", id, err)
		}
		sessions[id] = sess
	}
	return &State{file: file, device: dev, sessions: sessions}, nil
}

// Marshal encodes the state into JSON.
func (s *State) Marshal() ([]byte, error) {
	devState, err := s.device.Export()
	if err != nil {
		return nil, err
	}
	s.file.Device = devState
	if len(s.sessions) == 0 {
		s.file.Sessions = nil
	} else {
		sessions := make(map[string]*cryptocore.SessionStateSnapshot, len(s.sessions))
		for id, sess := range s.sessions {
			snap, err := cryptocore.ExportSession(sess)
			if err != nil {
				return nil, fmt.Errorf("export session %s: %w", id, err)
			}
			sessions[id] = snap
		}
		s.file.Sessions = sessions
	}
	return json.MarshalIndent(s.file, "", "  ")
}

// Clone returns a deep copy of the state, preserving the configured path.
func (s *State) Clone() (*State, error) {
	data, err := s.Marshal()
	if err != nil {
		return nil, err
	}
	clone, err := LoadStateFromJSON(data)
	if err != nil {
		return nil, err
	}
	clone.path = s.path
	return clone, nil
}

// PrepareSend encrypts plaintext for the given conversation and recipient.
func (s *State) PrepareSend(convID, toID uuid.UUID, plaintext string) (*sendRequest, error) {
	opts := &sendOptions{convID: convID, toID: toID, plaintext: plaintext}
	sess, handshake, err := ensureSession(s, convID, toID)
	if err != nil {
		return nil, err
	}
	return buildSendRequest(s.file.DeviceID, opts, sess, handshake)
}

// HandleEnvelope decrypts an inbound envelope and updates session state.
func (s *State) HandleEnvelope(env *InboundEnvelope) (string, error) {
	return handleInbound(env, s)
}

// DeviceID exposes the active device identifier.
func (s *State) DeviceID() string { return s.file.DeviceID }

// UserID exposes the registered user identifier.
func (s *State) UserID() string { return s.file.UserID }

// KeysBaseURL returns the configured key service base URL.
func (s *State) KeysBaseURL() string { return s.file.KeysBaseURL }

// MessagesBaseURL returns the configured message service base URL.
func (s *State) MessagesBaseURL() string { return s.file.MessagesBaseURL }

// SetPath assigns the persistence path used by Save.
func (s *State) SetPath(path string) { s.path = path }
