package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	cryptocore "cryptocore"
)

const (
	defaultStatePath   = "msgctl-state.json"
	defaultKeysBaseURL = "http://localhost:8080"
	defaultMsgBaseURL  = "http://localhost:8080"
)

type stateFile struct {
	UserID          string                                      `json:"user_id"`
	DeviceID        string                                      `json:"device_id"`
	KeysBaseURL     string                                      `json:"keys_base_url"`
	MessagesBaseURL string                                      `json:"messages_base_url"`
	Device          *cryptocore.DeviceState                     `json:"device"`
	Sessions        map[string]*cryptocore.SessionStateSnapshot `json:"sessions,omitempty"`
}

type runtimeState struct {
	path     string
	file     stateFile
	device   *cryptocore.Device
	sessions map[string]*cryptocore.SessionState
}

type registerDeviceRequest struct {
	UserID               string `json:"userId"`
	DeviceID             string `json:"deviceId"`
	IdentityKey          string `json:"identityKey"`
	IdentitySignatureKey string `json:"identitySignatureKey"`
	SignedPreKey         struct {
		PublicKey string    `json:"publicKey"`
		Signature string    `json:"signature"`
		CreatedAt time.Time `json:"createdAt"`
	} `json:"signedPreKey"`
	OneTimePreKeys []struct {
		ID        string `json:"id"`
		PublicKey string `json:"publicKey"`
	} `json:"oneTimePreKeys"`
}

type registerDeviceResponse struct {
	UserID         string `json:"userId"`
	DeviceID       string `json:"deviceId"`
	OneTimePreKeys int    `json:"oneTimePreKeys"`
}

type preKeyBundleResponse struct {
	DeviceID             string `json:"deviceId"`
	IdentityKey          string `json:"identityKey"`
	IdentitySignatureKey string `json:"identitySignatureKey"`
	SignedPreKey         struct {
		PublicKey string    `json:"publicKey"`
		Signature string    `json:"signature"`
		CreatedAt time.Time `json:"createdAt"`
	} `json:"signedPreKey"`
	OneTimePreKey *struct {
		ID        string `json:"id"`
		PublicKey string `json:"publicKey"`
	} `json:"oneTimePreKey"`
}

type sendRequest struct {
	ConvID       string          `json:"conv_id"`
	FromDeviceID string          `json:"from_device_id"`
	ToDeviceID   string          `json:"to_device_id"`
	Ciphertext   string          `json:"ciphertext"`
	Header       json.RawMessage `json:"header"`
}

type headerPayload struct {
	Handshake *handshakePayload `json:"handshake,omitempty"`
	Ratchet   ratchetPayload    `json:"ratchet"`
}

type handshakePayload struct {
	IdentityKey          string  `json:"identityKey"`
	IdentitySignatureKey string  `json:"identitySignatureKey"`
	EphemeralKey         string  `json:"ephemeralKey"`
	OneTimePrekeyID      *uint32 `json:"oneTimePrekeyId,omitempty"`
}

type ratchetPayload struct {
	DHPublic string `json:"dhPublic"`
	PN       uint32 `json:"pn"`
	N        uint32 `json:"n"`
	Nonce    string `json:"nonce"`
}

type inboundEnvelope struct {
	ID           string          `json:"id"`
	ConvID       string          `json:"conv_id"`
	FromDeviceID string          `json:"from_device_id"`
	ToDeviceID   string          `json:"to_device_id"`
	Ciphertext   string          `json:"ciphertext"`
	Header       json.RawMessage `json:"header"`
	SentAt       time.Time       `json:"sent_at"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "init":
		err = runInit(args)
	case "send":
		err = runSend(args)
	case "listen":
		err = runListen(args)
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
	fmt.Fprintln(os.Stderr, "  init      Initialize a device and register with the key service")
	fmt.Fprintln(os.Stderr, "  send      Encrypt and send a message")
	fmt.Fprintln(os.Stderr, "  listen    Connect to the message service and receive messages")
	os.Exit(2)
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	statePath := fs.String("state", getenv("MSGCTL_STATE_PATH", defaultStatePath), "state file path")
	keysURL := fs.String("keys-url", getenv("MSGCTL_KEYS_URL", defaultKeysBaseURL), "keys service base URL")
	msgsURL := fs.String("messages-url", getenv("MSGCTL_MESSAGES_URL", defaultMsgBaseURL), "messages service base URL")
	userID := fs.String("user", "", "existing user ID (optional)")
	deviceID := fs.String("device", "", "existing device ID (optional)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := os.Stat(*statePath); err == nil {
		return fmt.Errorf("state file already exists at %s", *statePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	dev, err := cryptocore.GenerateIdentityKeypair()
	if err != nil {
		return fmt.Errorf("generate identity: %w", err)
	}
	bundle, err := dev.PublishPrekeyBundle(0)
	if err != nil {
		return fmt.Errorf("publish bundle: %w", err)
	}

	req := registerDeviceRequest{
		UserID:               strings.TrimSpace(*userID),
		DeviceID:             strings.TrimSpace(*deviceID),
		IdentityKey:          base64.StdEncoding.EncodeToString(bundle.IdentityKey[:]),
		IdentitySignatureKey: base64.StdEncoding.EncodeToString(bundle.IdentitySignatureKey),
	}
	req.SignedPreKey.PublicKey = base64.StdEncoding.EncodeToString(bundle.SignedPrekey[:])
	req.SignedPreKey.Signature = base64.StdEncoding.EncodeToString(bundle.SignedPrekeySig)
	req.SignedPreKey.CreatedAt = time.Now().UTC()

	body, err := json.Marshal(req)
	if err != nil {
		return err
	}

	endpoint := joinURL(*keysURL, "/keys/device/register")
	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
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
	var regResp registerDeviceResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return err
	}
	state := &runtimeState{
		path: *statePath,
		file: stateFile{
			UserID:          regResp.UserID,
			DeviceID:        regResp.DeviceID,
			KeysBaseURL:     normalizeBaseURL(*keysURL),
			MessagesBaseURL: normalizeBaseURL(*msgsURL),
		},
		device:   dev,
		sessions: make(map[string]*cryptocore.SessionState),
	}
	if err := state.save(); err != nil {
		return err
	}
	fmt.Printf("device registered: user=%s device=%s\n", regResp.UserID, regResp.DeviceID)
	return nil
}

func runSend(args []string) error {
	fs := flag.NewFlagSet("send", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	statePath := fs.String("state", getenv("MSGCTL_STATE_PATH", defaultStatePath), "state file path")
	convIDStr := fs.String("conv", "", "conversation UUID")
	toDevice := fs.String("to", "", "recipient device UUID")
	message := fs.String("message", "", "message plaintext (if empty, read stdin)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*convIDStr) == "" {
		return fmt.Errorf("conversation id is required")
	}
	if strings.TrimSpace(*toDevice) == "" {
		return fmt.Errorf("recipient device id is required")
	}
	convID, err := uuid.Parse(*convIDStr)
	if err != nil {
		return fmt.Errorf("invalid conversation id: %w", err)
	}
	toID, err := uuid.Parse(*toDevice)
	if err != nil {
		return fmt.Errorf("invalid recipient device id: %w", err)
	}
	plaintext := *message
	if plaintext == "" {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		plaintext = string(data)
	}
	if plaintext == "" {
		return fmt.Errorf("message must not be empty")
	}
	state, err := loadState(*statePath)
	if err != nil {
		return err
	}
	sess, ok := state.sessions[convID.String()]
	var handshake *cryptocore.HandshakeMessage
	if !ok {
		bundle, err := fetchBundle(state.file.KeysBaseURL, toID)
		if err != nil {
			return err
		}
		sess, handshake, err = state.device.InitSession(bundle)
		if err != nil {
			return fmt.Errorf("init session: %w", err)
		}
		state.sessions[convID.String()] = sess
	}
	ciphertext, header, err := cryptocore.Encrypt(sess, []byte(plaintext))
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}
	headerJSON, err := buildHeaderJSON(header, handshake)
	if err != nil {
		return err
	}
	req := sendRequest{
		ConvID:       convID.String(),
		FromDeviceID: state.file.DeviceID,
		ToDeviceID:   toID.String(),
		Ciphertext:   base64.StdEncoding.EncodeToString(ciphertext),
		Header:       headerJSON,
	}
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	endpoint := joinURL(state.file.MessagesBaseURL, "/messages/send")
	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		if len(data) == 0 {
			data = []byte(resp.Status)
		}
		return fmt.Errorf("send failed: %s", strings.TrimSpace(string(data)))
	}
	if err := state.save(); err != nil {
		return err
	}
	fmt.Println("message queued")
	return nil
}

func runListen(args []string) error {
	fs := flag.NewFlagSet("listen", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	statePath := fs.String("state", getenv("MSGCTL_STATE_PATH", defaultStatePath), "state file path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, err := loadState(*statePath)
	if err != nil {
		return err
	}
	wsURL, err := websocketURL(state.file.MessagesBaseURL, state.file.DeviceID)
	if err != nil {
		return err
	}
	conn, err := dialWebsocket(wsURL)
	if err != nil {
		return err
	}
	defer conn.Close()
	writer := bufio.NewWriter(os.Stdout)
	defer writer.Flush()

	for {
		payload, err := conn.ReadText()
		if err != nil {
			return err
		}
		var env inboundEnvelope
		if err := json.Unmarshal(payload, &env); err != nil {
			fmt.Fprintf(os.Stderr, "invalid envelope: %v\n", err)
			continue
		}
		plaintext, err := handleInbound(&env, state)
		if err != nil {
			fmt.Fprintf(os.Stderr, "decrypt failed: %v\n", err)
			continue
		}
		fmt.Fprintf(writer, "[%s] %s -> %s: %s\n", env.SentAt.Format(time.RFC3339), env.FromDeviceID, env.ToDeviceID, plaintext)
		writer.Flush()
		if err := state.save(); err != nil {
			return err
		}
	}
}

func handleInbound(env *inboundEnvelope, state *runtimeState) (string, error) {
	var header headerPayload
	if err := json.Unmarshal(env.Header, &header); err != nil {
		return "", fmt.Errorf("decode header: %w", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}
	convID := env.ConvID
	sess, ok := state.sessions[convID]
	if !ok {
		if header.Handshake == nil {
			return "", fmt.Errorf("missing handshake for new session")
		}
		hs, err := payloadToHandshake(header.Handshake)
		if err != nil {
			return "", err
		}
		sess, err = state.device.AcceptSession(hs)
		if err != nil {
			return "", fmt.Errorf("accept session: %w", err)
		}
		state.sessions[convID] = sess
	}
	msgHeader, err := payloadToMessageHeader(&header.Ratchet)
	if err != nil {
		return "", err
	}
	plaintext, err := cryptocore.Decrypt(sess, ciphertext, msgHeader)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plaintext), nil
}

func fetchBundle(base string, deviceID uuid.UUID) (*cryptocore.PrekeyBundle, error) {
	endpoint := joinURL(base, "/keys/bundle") + "?device_id=" + url.QueryEscape(deviceID.String())
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		if len(data) == 0 {
			data = []byte(resp.Status)
		}
		return nil, fmt.Errorf("bundle request failed: %s", strings.TrimSpace(string(data)))
	}
	var bundle preKeyBundleResponse
	if err := json.NewDecoder(resp.Body).Decode(&bundle); err != nil {
		return nil, err
	}
	return convertBundle(&bundle)
}

func convertBundle(resp *preKeyBundleResponse) (*cryptocore.PrekeyBundle, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil bundle response")
	}
	var out cryptocore.PrekeyBundle
	key, err := decode32(resp.IdentityKey)
	if err != nil {
		return nil, fmt.Errorf("decode identity key: %w", err)
	}
	out.IdentityKey = key
	sigKey, err := base64.StdEncoding.DecodeString(resp.IdentitySignatureKey)
	if err != nil {
		return nil, fmt.Errorf("decode identity signature key: %w", err)
	}
	out.IdentitySignatureKey = sigKey
	spk, err := decode32(resp.SignedPreKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("decode signed prekey: %w", err)
	}
	out.SignedPrekey = spk
	sig, err := base64.StdEncoding.DecodeString(resp.SignedPreKey.Signature)
	if err != nil {
		return nil, fmt.Errorf("decode signed prekey sig: %w", err)
	}
	out.SignedPrekeySig = sig
	if resp.OneTimePreKey != nil {
		if id, err := parseUint32(resp.OneTimePreKey.ID); err == nil {
			pk, err := decode32(resp.OneTimePreKey.PublicKey)
			if err != nil {
				return nil, fmt.Errorf("decode one-time prekey: %w", err)
			}
			out.OneTimePrekeys = []cryptocore.OneTimePrekey{{ID: id, Public: pk}}
		}
	}
	return &out, nil
}

func buildHeaderJSON(header *cryptocore.MessageHeader, handshake *cryptocore.HandshakeMessage) (json.RawMessage, error) {
	if header == nil {
		return nil, fmt.Errorf("nil message header")
	}
	hp := headerPayload{
		Ratchet: ratchetPayload{
			DHPublic: base64.StdEncoding.EncodeToString(header.DHPublic[:]),
			PN:       header.PN,
			N:        header.N,
			Nonce:    base64.StdEncoding.EncodeToString(header.Nonce[:]),
		},
	}
	if handshake != nil {
		hp.Handshake = &handshakePayload{
			IdentityKey:          base64.StdEncoding.EncodeToString(handshake.IdentityKey[:]),
			IdentitySignatureKey: base64.StdEncoding.EncodeToString(handshake.IdentitySignatureKey),
			EphemeralKey:         base64.StdEncoding.EncodeToString(handshake.EphemeralKey[:]),
			OneTimePrekeyID:      handshake.OneTimePrekeyID,
		}
	}
	data, err := json.Marshal(hp)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func payloadToMessageHeader(p *ratchetPayload) (*cryptocore.MessageHeader, error) {
	if p == nil {
		return nil, fmt.Errorf("nil ratchet payload")
	}
	dh, err := decode32(p.DHPublic)
	if err != nil {
		return nil, fmt.Errorf("decode dh public: %w", err)
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(p.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}
	if len(nonceBytes) != 12 {
		return nil, fmt.Errorf("invalid nonce length %d", len(nonceBytes))
	}
	var nonce [12]byte
	copy(nonce[:], nonceBytes)
	header := &cryptocore.MessageHeader{
		DHPublic: dh,
		PN:       p.PN,
		N:        p.N,
		Nonce:    nonce,
	}
	return header, nil
}

func payloadToHandshake(p *handshakePayload) (*cryptocore.HandshakeMessage, error) {
	if p == nil {
		return nil, fmt.Errorf("nil handshake payload")
	}
	identity, err := decode32(p.IdentityKey)
	if err != nil {
		return nil, fmt.Errorf("decode handshake identity: %w", err)
	}
	sigKey, err := base64.StdEncoding.DecodeString(p.IdentitySignatureKey)
	if err != nil {
		return nil, fmt.Errorf("decode handshake signature key: %w", err)
	}
	eph, err := decode32(p.EphemeralKey)
	if err != nil {
		return nil, fmt.Errorf("decode handshake ephemeral: %w", err)
	}
	msg := &cryptocore.HandshakeMessage{
		IdentityKey:          identity,
		IdentitySignatureKey: sigKey,
		EphemeralKey:         eph,
		OneTimePrekeyID:      p.OneTimePrekeyID,
	}
	return msg, nil
}

func loadState(path string) (*runtimeState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file stateFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if file.Device == nil {
		return nil, fmt.Errorf("state file missing device")
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
	return &runtimeState{path: path, file: file, device: dev, sessions: sessions}, nil
}

func (s *runtimeState) save() error {
	devState, err := s.device.Export()
	if err != nil {
		return err
	}
	s.file.Device = devState
	if len(s.sessions) == 0 {
		s.file.Sessions = nil
	} else {
		sessions := make(map[string]*cryptocore.SessionStateSnapshot, len(s.sessions))
		for id, sess := range s.sessions {
			snap, err := cryptocore.ExportSession(sess)
			if err != nil {
				return fmt.Errorf("export session %s: %w", id, err)
			}
			sessions[id] = snap
		}
		s.file.Sessions = sessions
	}
	data, err := json.MarshalIndent(s.file, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

const (
	wsOpText = 0x1
	wsOpPing = 0x9
	wsOpPong = 0xA
)

type wsClientConn struct {
	conn net.Conn
	rw   *bufio.ReadWriter
	mu   sync.Mutex
}

func dialWebsocket(rawURL string) (*wsClientConn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	var conn net.Conn
	host := u.Host
	switch strings.ToLower(u.Scheme) {
	case "ws":
		if !strings.Contains(host, ":") {
			host += ":80"
		}
		conn, err = net.Dial("tcp", host)
	case "wss":
		if !strings.Contains(host, ":") {
			host += ":443"
		}
		conn, err = tls.Dial("tcp", host, &tls.Config{InsecureSkipVerify: true})
	default:
		return nil, fmt.Errorf("unsupported websocket scheme %s", u.Scheme)
	}
	if err != nil {
		return nil, err
	}
	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		conn.Close()
		return nil, err
	}
	key := base64.StdEncoding.EncodeToString(keyBytes)
	path := u.RequestURI()
	if path == "" {
		path = "/"
	}
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, u.Host, key)
	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
	if _, err := rw.WriteString(req); err != nil {
		conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		conn.Close()
		return nil, err
	}
	status, err := rw.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, err
	}
	if !strings.Contains(status, "101") {
		conn.Close()
		return nil, fmt.Errorf("websocket handshake failed: %s", strings.TrimSpace(status))
	}
	expected := computeAccept(key)
	var accept string
	for {
		line, err := rw.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "Sec-WebSocket-Accept") {
			accept = strings.TrimSpace(parts[1])
		}
	}
	if accept == "" || accept != expected {
		conn.Close()
		return nil, fmt.Errorf("websocket handshake validation failed")
	}
	return &wsClientConn{conn: conn, rw: rw}, nil
}

func (c *wsClientConn) ReadText() ([]byte, error) {
	for {
		first, err := c.rw.ReadByte()
		if err != nil {
			return nil, err
		}
		fin := first&0x80 != 0
		opcode := first & 0x0F
		second, err := c.rw.ReadByte()
		if err != nil {
			return nil, err
		}
		masked := second&0x80 != 0
		length := int(second & 0x7F)
		switch length {
		case 126:
			var ext uint16
			if err := binary.Read(c.rw, binary.BigEndian, &ext); err != nil {
				return nil, err
			}
			length = int(ext)
		case 127:
			var ext uint64
			if err := binary.Read(c.rw, binary.BigEndian, &ext); err != nil {
				return nil, err
			}
			if ext > (1<<31 - 1) {
				return nil, fmt.Errorf("frame too large")
			}
			length = int(ext)
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(c.rw, mask[:]); err != nil {
				return nil, err
			}
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(c.rw, payload); err != nil {
			return nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i%4]
			}
		}
		if !fin {
			return nil, fmt.Errorf("fragmented frames not supported")
		}
		switch opcode {
		case wsOpText:
			return payload, nil
		case 0x8:
			return nil, io.EOF
		case wsOpPing:
			if err := c.writeFrame(wsOpPong, payload); err != nil {
				return nil, err
			}
		case wsOpPong:
			// ignore
		default:
			// ignore other opcodes
		}
	}
}

func (c *wsClientConn) writeFrame(opcode byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if err := c.rw.WriteByte(0x80 | opcode); err != nil {
		return err
	}
	length := len(payload)
	switch {
	case length <= 125:
		if err := c.rw.WriteByte(0x80 | byte(length)); err != nil {
			return err
		}
	case length < 65536:
		if err := c.rw.WriteByte(0x80 | 126); err != nil {
			return err
		}
		if err := binary.Write(c.rw, binary.BigEndian, uint16(length)); err != nil {
			return err
		}
	default:
		if err := c.rw.WriteByte(0x80 | 127); err != nil {
			return err
		}
		if err := binary.Write(c.rw, binary.BigEndian, uint64(length)); err != nil {
			return err
		}
	}
	var mask [4]byte
	if _, err := rand.Read(mask[:]); err != nil {
		return err
	}
	if _, err := c.rw.Write(mask[:]); err != nil {
		return err
	}
	masked := make([]byte, length)
	for i, b := range payload {
		masked[i] = b ^ mask[i%4]
	}
	if _, err := c.rw.Write(masked); err != nil {
		return err
	}
	return c.rw.Flush()
}

func (c *wsClientConn) Close() error {
	return c.conn.Close()
}

func computeAccept(key string) string {
	const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	sum := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func decode32(s string) ([32]byte, error) {
	data, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return [32]byte{}, err
	}
	if len(data) != 32 {
		return [32]byte{}, fmt.Errorf("expected 32 bytes, got %d", len(data))
	}
	var out [32]byte
	copy(out[:], data)
	return out, nil
}

func parseUint32(v string) (uint32, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, fmt.Errorf("empty value")
	}
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(n), nil
}

func joinURL(base, path string) string {
	base = normalizeBaseURL(base)
	if strings.HasSuffix(base, "/") {
		base = strings.TrimSuffix(base, "/")
	}
	return base + path
}

func normalizeBaseURL(in string) string {
	return strings.TrimRight(strings.TrimSpace(in), "/")
}

func websocketURL(base, deviceID string) (string, error) {
	base = normalizeBaseURL(base)
	if base == "" {
		return "", fmt.Errorf("messages base URL missing in state")
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(u.Scheme) {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported scheme %s", u.Scheme)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/ws"
	q := u.Query()
	q.Set("device_id", deviceID)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
