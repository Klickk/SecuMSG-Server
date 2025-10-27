package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"keys/internal/dto"

	"github.com/google/uuid"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "register":
		err = runRegister(args)
	case "bundle":
		err = runBundle(args)
	default:
		usage()
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  register   Register a device with the key service")
	fmt.Fprintln(os.Stderr, "  bundle     Fetch a pre-key bundle for a device")
	os.Exit(2)
}

func runRegister(args []string) error {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	baseURL := fs.String("base-url", getenv("KEYCTL_BASE_URL", "http://localhost:8082"), "key service base URL")
	userID := fs.String("user", "", "user UUID (optional; generated if empty)")
	deviceID := fs.String("device", "", "device UUID (optional; generated if empty)")
	identity := fs.String("identity", "", "identity key (base64; generated if empty)")
	signed := fs.String("signed", "", "signed pre-key (base64; generated if empty)")
	signature := fs.String("signature", "", "signed pre-key signature (base64; generated if empty)")
	count := fs.Int("count", 5, "number of one-time pre-keys to generate")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *count < 0 {
		return fmt.Errorf("count must be non-negative")
	}

	payload := dto.RegisterDeviceRequest{
		UserID:      strings.TrimSpace(*userID),
		DeviceID:    strings.TrimSpace(*deviceID),
		IdentityKey: *identity,
		SignedPreKey: dto.SignedPreKey{
			PublicKey: *signed,
			Signature: *signature,
			CreatedAt: time.Now().UTC(),
		},
	}

	if payload.IdentityKey == "" {
		key, err := randomKey(32)
		if err != nil {
			return err
		}
		payload.IdentityKey = key
	}
	if payload.SignedPreKey.PublicKey == "" {
		key, err := randomKey(32)
		if err != nil {
			return err
		}
		payload.SignedPreKey.PublicKey = key
	}
	if payload.SignedPreKey.Signature == "" {
		sig, err := randomKey(64)
		if err != nil {
			return err
		}
		payload.SignedPreKey.Signature = sig
	}

	payload.OneTimePreKeys = make([]dto.OneTimePreKey, *count)
	for i := 0; i < *count; i++ {
		key, err := randomKey(32)
		if err != nil {
			return err
		}
		payload.OneTimePreKeys[i] = dto.OneTimePreKey{
			ID:        uuid.New().String(),
			PublicKey: key,
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(*baseURL, "/")+"/keys/device/register", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		if len(data) == 0 {
			data = []byte(resp.Status)
		}
		return fmt.Errorf("register request failed: %s", strings.TrimSpace(string(data)))
	}

	var registerResp dto.RegisterDeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&registerResp); err != nil {
		return err
	}

	if payload.UserID == "" {
		payload.UserID = registerResp.UserID
	}
	if payload.DeviceID == "" {
		payload.DeviceID = registerResp.DeviceID
	}

	output := struct {
		Request  dto.RegisterDeviceRequest  `json:"request"`
		Response dto.RegisterDeviceResponse `json:"response"`
	}{payload, registerResp}

	return printJSON(output)
}

func runBundle(args []string) error {
	fs := flag.NewFlagSet("bundle", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	baseURL := fs.String("base-url", getenv("KEYCTL_BASE_URL", "http://localhost:8082"), "key service base URL")
	deviceID := fs.String("device", "", "device UUID")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*deviceID) == "" {
		return fmt.Errorf("device id is required")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("%s/keys/bundle?device_id=%s", strings.TrimRight(*baseURL, "/"), *deviceID)
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		if len(data) == 0 {
			data = []byte(resp.Status)
		}
		return fmt.Errorf("bundle request failed: %s", strings.TrimSpace(string(data)))
	}

	var bundle dto.PreKeyBundleResponse
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		return err
	}

	return printJSON(bundle)
}

func randomKey(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
