package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Claims struct {
	Valid            bool
	UserID           uuid.UUID
	SessionID        uuid.UUID
	TokenDeviceID    *uuid.UUID
	DeviceAuthorized bool
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://localhost:8081"
	}
	return &Client{
		baseURL: base,
		http:    &http.Client{Timeout: 5 * time.Second},
	}
}

// Verify checks the provided JWT via the auth service verify endpoint.
// If deviceID is non-nil and not uuid.Nil, auth will also assert the device belongs to the user.
func (c *Client) Verify(ctx context.Context, token string, deviceID uuid.UUID) (Claims, error) {
	payload := map[string]string{"token": strings.TrimSpace(token)}
	if deviceID != uuid.Nil {
		payload["deviceId"] = deviceID.String()
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return Claims{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/auth/verify", bytes.NewReader(data))
	if err != nil {
		return Claims{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Claims{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Claims{}, fmt.Errorf("auth verify failed: %s", resp.Status)
	}

	var body struct {
		Valid            bool   `json:"valid"`
		UserID           string `json:"userId"`
		SessionID        string `json:"sessionId"`
		TokenDeviceID    string `json:"tokenDeviceId"`
		DeviceAuthorized bool   `json:"deviceAuthorized"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Claims{}, err
	}
	if !body.Valid {
		return Claims{Valid: false, DeviceAuthorized: body.DeviceAuthorized}, nil
	}

	userID, err := uuid.Parse(body.UserID)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid user id from verify response")
	}
	sessionID, err := uuid.Parse(body.SessionID)
	if err != nil {
		return Claims{}, fmt.Errorf("invalid session id from verify response")
	}
	var tokenDeviceID *uuid.UUID
	if strings.TrimSpace(body.TokenDeviceID) != "" {
		if did, err := uuid.Parse(body.TokenDeviceID); err == nil {
			tokenDeviceID = &did
		}
	}

	return Claims{
		Valid:            true,
		UserID:           userID,
		SessionID:        sessionID,
		TokenDeviceID:    tokenDeviceID,
		DeviceAuthorized: body.DeviceAuthorized,
	}, nil
}
