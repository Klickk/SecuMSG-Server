package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"keys/internal/dto"
	"net/http"
	"os"
	"strings"
	"time"

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

type registerOpts struct {
	baseURL   string
	userID    string
	deviceID  string
	identity  string
	signed    string
	signature string
	count     int
}

func runRegister(args []string) error {
	opts, err := parseRegisterFlags(args)
	if err != nil {
		return err
	}
	payload, err := buildRegisterPayload(opts)
	if err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := strings.TrimRight(opts.baseURL, "/") + "/keys/device/register"
	resp, err := postJSON(endpoint, body)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close response body: %v\n", cerr)
		}
	}()

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

	// Fill back generated IDs so the printed request matches the effective state.
	if payload.UserID == "" {
		payload.UserID = registerResp.UserID
	}
	if payload.DeviceID == "" {
		payload.DeviceID = registerResp.DeviceID
	}

	out := struct {
		Request  dto.RegisterDeviceRequest  `json:"request"`
		Response dto.RegisterDeviceResponse `json:"response"`
	}{payload, registerResp}

	return printJSON(out)
}

func parseRegisterFlags(args []string) (registerOpts, error) {
	fs := flag.NewFlagSet("register", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var o registerOpts
	fs.StringVar(&o.baseURL, "base-url", getenv("KEYCTL_BASE_URL", "http://localhost:8082"), "key service base URL")
	fs.StringVar(&o.userID, "user", "", "user UUID (optional; generated if empty)")
	fs.StringVar(&o.deviceID, "device", "", "device UUID (optional; generated if empty)")
	fs.StringVar(&o.identity, "identity", "", "identity key (base64; generated if empty)")
	fs.StringVar(&o.signed, "signed", "", "signed pre-key (base64; generated if empty)")
	fs.StringVar(&o.signature, "signature", "", "signed pre-key signature (base64; generated if empty)")
	fs.IntVar(&o.count, "count", 5, "number of one-time pre-keys to generate")

	if err := fs.Parse(args); err != nil {
		return registerOpts{}, err
	}
	if o.count < 0 {
		return registerOpts{}, fmt.Errorf("count must be non-negative")
	}
	return o, nil
}

func buildRegisterPayload(o registerOpts) (dto.RegisterDeviceRequest, error) {
	p := dto.RegisterDeviceRequest{
		UserID:      strings.TrimSpace(o.userID),
		DeviceID:    strings.TrimSpace(o.deviceID),
		IdentityKey: strings.TrimSpace(o.identity),
		SignedPreKey: dto.SignedPreKey{
			PublicKey: strings.TrimSpace(o.signed),
			Signature: strings.TrimSpace(o.signature),
			CreatedAt: time.Now().UTC(),
		},
	}

	var err error
	if p.IdentityKey == "" {
		if p.IdentityKey, err = randomKey(32); err != nil {
			return dto.RegisterDeviceRequest{}, err
		}
	}
	if p.SignedPreKey.PublicKey == "" {
		if p.SignedPreKey.PublicKey, err = randomKey(32); err != nil {
			return dto.RegisterDeviceRequest{}, err
		}
	}
	if p.SignedPreKey.Signature == "" {
		if p.SignedPreKey.Signature, err = randomKey(64); err != nil {
			return dto.RegisterDeviceRequest{}, err
		}
	}

	p.OneTimePreKeys = make([]dto.OneTimePreKey, o.count)
	for i := range p.OneTimePreKeys {
		key, kerr := randomKey(32)
		if kerr != nil {
			return dto.RegisterDeviceRequest{}, kerr
		}
		p.OneTimePreKeys[i] = dto.OneTimePreKey{
			ID:        uuid.New().String(),
			PublicKey: key,
		}
	}
	return p, nil
}

func postJSON(url string, body []byte) (*http.Response, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
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
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to close response body: %v\n", cerr)
		}
	}()

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
