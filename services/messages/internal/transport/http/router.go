package http

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"messages/internal/observability/middleware"
	"messages/internal/service"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Handler struct {
	svc   *service.Service
	poll  time.Duration
	batch int
}

type sendRequest struct {
	ConvID       string          `json:"conv_id"`
	FromDeviceID string          `json:"from_device_id"`
	ToDeviceID   string          `json:"to_device_id"`
	Ciphertext   string          `json:"ciphertext"`
	Header       json.RawMessage `json:"header"`
}

type sendResponse struct {
	ID         string    `json:"id"`
	ConvID     string    `json:"conv_id"`
	ToDeviceID string    `json:"to_device_id"`
	SentAt     time.Time `json:"sent_at"`
}

type outboundEnvelope struct {
	ID           string          `json:"id"`
	ConvID       string          `json:"conv_id"`
	FromDeviceID string          `json:"from_device_id"`
	ToDeviceID   string          `json:"to_device_id"`
	Ciphertext   string          `json:"ciphertext"`
	Header       json.RawMessage `json:"header"`
	SentAt       time.Time       `json:"sent_at"`
}

func NewRouter(svc *service.Service, poll time.Duration, batch int) http.Handler {
	if poll <= 0 {
		poll = 500 * time.Millisecond
	}
	if batch <= 0 {
		batch = 50
	}
	h := &Handler{svc: svc, poll: poll, batch: batch}
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/messages/send", h.handleSend)
	mux.HandleFunc("/messages/conversations", h.handleConversations)
	mux.HandleFunc("/messages/history", h.handleHistory)
	mux.HandleFunc("/ws", h.handleWS)
	mux.HandleFunc("/client/init", h.handleClientInit)
	mux.HandleFunc("/client/send", h.handleClientSend)
	mux.HandleFunc("/client/envelope", h.handleClientEnvelope)
	return mux
}

func (h *Handler) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req sendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	convID, err := uuid.Parse(req.ConvID)
	if err != nil {
		http.Error(w, "invalid conv_id", http.StatusBadRequest)
		return
	}
	fromID, err := uuid.Parse(req.FromDeviceID)
	if err != nil {
		http.Error(w, "invalid from_device_id", http.StatusBadRequest)
		return
	}
	toID, err := uuid.Parse(req.ToDeviceID)
	if err != nil {
		http.Error(w, "invalid to_device_id", http.StatusBadRequest)
		return
	}
	if len(req.Header) == 0 || !json.Valid(req.Header) {
		http.Error(w, "invalid header", http.StatusBadRequest)
		return
	}
	ciphertext, err := base64.StdEncoding.DecodeString(req.Ciphertext)
	if err != nil {
		http.Error(w, "invalid ciphertext", http.StatusBadRequest)
		return
	}
	msg, err := h.svc.Enqueue(r.Context(), service.SendInput{
		ConvID:       convID,
		FromDeviceID: fromID,
		ToDeviceID:   toID,
		Ciphertext:   ciphertext,
		Header:       req.Header,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidRequest) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}
	resp := sendResponse{
		ID:         msg.ID.String(),
		ConvID:     msg.ConvID.String(),
		ToDeviceID: msg.ToDeviceID.String(),
		SentAt:     msg.SentAt,
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (h *Handler) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	params := r.URL.Query()
	deviceParam := params.Get("device_id")
	if deviceParam == "" {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}
	deviceID, err := uuid.Parse(deviceParam)
	if err != nil {
		http.Error(w, "invalid device_id", http.StatusBadRequest)
		return
	}

	var convID uuid.UUID
	if convParam := params.Get("conv_id"); convParam != "" {
		convID, err = uuid.Parse(convParam)
		if err != nil {
			http.Error(w, "invalid conv_id", http.StatusBadRequest)
			return
		}
	}

	var since time.Time
	if sinceParam := params.Get("since"); sinceParam != "" {
		since, err = time.Parse(time.RFC3339Nano, sinceParam)
		if err != nil {
			http.Error(w, "invalid since timestamp", http.StatusBadRequest)
			return
		}
	}

	limit := h.batch
	if limitParam := params.Get("limit"); limitParam != "" {
		if v, err := strconv.Atoi(limitParam); err == nil && v > 0 {
			limit = v
		}
	}

	msgs, err := h.svc.History(r.Context(), deviceID, since, convID, limit)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidRequest) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	resp := struct {
		Messages []outboundEnvelope `json:"messages"`
	}{Messages: make([]outboundEnvelope, 0, len(msgs))}

	for _, m := range msgs {
		resp.Messages = append(resp.Messages, outboundEnvelope{
			ID:           m.ID.String(),
			ConvID:       m.ConvID.String(),
			FromDeviceID: m.FromDeviceID.String(),
			ToDeviceID:   m.ToDeviceID.String(),
			Ciphertext:   base64.StdEncoding.EncodeToString(m.Ciphertext),
			Header:       append(json.RawMessage(nil), m.Header...),
			SentAt:       m.SentAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleConversations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	deviceParam := strings.TrimSpace(r.URL.Query().Get("device_id"))
	if deviceParam == "" {
		http.Error(w, "device_id is required", http.StatusBadRequest)
		return
	}

	deviceID, err := uuid.Parse(deviceParam)
	if err != nil {
		http.Error(w, "invalid device_id", http.StatusBadRequest)
		return
	}

	ids, err := h.svc.Conversations(r.Context(), deviceID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidRequest) {
			status = http.StatusBadRequest
		}
		http.Error(w, err.Error(), status)
		return
	}

	resp := struct {
		Conversations []string `json:"conversations"`
	}{Conversations: make([]string, 0, len(ids))}

	for _, id := range ids {
		resp.Conversations = append(resp.Conversations, id.String())
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	deviceParam := r.URL.Query().Get("device_id")
	if deviceParam == "" {
		http.Error(w, "missing device_id", http.StatusBadRequest)
		return
	}
	deviceID, err := uuid.Parse(deviceParam)
	if err != nil {
		http.Error(w, "invalid device_id", http.StatusBadRequest)
		return
	}
	ws, err := acceptWebSocket(w, r)
	if err != nil {
		reqID := middleware.RequestIDFromContext(r.Context())
		traceID := middleware.TraceIDFromContext(r.Context())
		slog.Error("ws handshake", "error", err, "request_id", reqID, "trace_id", traceID)
		return
	}
	defer ws.close()

	ctx := r.Context()
	reqID := middleware.RequestIDFromContext(ctx)
	traceID := middleware.TraceIDFromContext(ctx)

	sendPending := func() error {
		msgs, err := h.svc.Pending(ctx, deviceID, h.batch)
		if err != nil {
			return err
		}
		if len(msgs) == 0 {
			return nil
		}
		ids := make([]uuid.UUID, 0, len(msgs))
		for _, m := range msgs {
			env := outboundEnvelope{
				ID:           m.ID.String(),
				ConvID:       m.ConvID.String(),
				FromDeviceID: m.FromDeviceID.String(),
				ToDeviceID:   m.ToDeviceID.String(),
				Ciphertext:   base64.StdEncoding.EncodeToString(m.Ciphertext),
				Header:       append(json.RawMessage(nil), m.Header...),
				SentAt:       m.SentAt,
			}
			data, err := json.Marshal(env)
			if err != nil {
				return err
			}
			if err := ws.writeFrame(opText, data); err != nil {
				return err
			}
			ids = append(ids, m.ID)
		}
		return h.svc.MarkDelivered(ctx, ids)
	}

	if err := sendPending(); err != nil {
		slog.Error("ws initial send", "error", err, "request_id", reqID, "trace_id", traceID)
		return
	}

	ticker := time.NewTicker(h.poll)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := sendPending(); err != nil {
				slog.Error("ws send", "error", err, "request_id", reqID, "trace_id", traceID)
				return
			}
			if err := ws.writeFrame(opPing, nil); err != nil {
				slog.Error("ws ping", "error", err, "request_id", reqID, "trace_id", traceID)
				return
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

const (
	opText = 0x1
	opPing = 0x9
)

type wsServerConn struct {
	conn net.Conn
	w    *bufio.Writer
	mu   sync.Mutex
}

func acceptWebSocket(w http.ResponseWriter, r *http.Request) (*wsServerConn, error) {
	if !strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
		http.Error(w, "bad request", http.StatusBadRequest)
		return nil, fmt.Errorf("missing upgrade header")
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "bad request", http.StatusBadRequest)
		return nil, fmt.Errorf("invalid upgrade value")
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return nil, fmt.Errorf("missing websocket key")
	}
	accept := computeAccept(key)
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "upgrade not supported", http.StatusInternalServerError)
		return nil, fmt.Errorf("hijacking not supported")
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	response := fmt.Sprintf("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept)
	if _, err := rw.WriteString(response); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &wsServerConn{conn: conn, w: bufio.NewWriter(conn)}, nil
}

func (c *wsServerConn) writeFrame(opcode byte, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	if err := c.w.WriteByte(0x80 | opcode); err != nil {
		return err
	}
	length := len(payload)
	switch {
	case length <= 125:
		if err := c.w.WriteByte(byte(length)); err != nil {
			return err
		}
	case length < 65536:
		if err := c.w.WriteByte(126); err != nil {
			return err
		}
		if err := binary.Write(c.w, binary.BigEndian, uint16(length)); err != nil {
			return err
		}
	default:
		if err := c.w.WriteByte(127); err != nil {
			return err
		}
		if err := binary.Write(c.w, binary.BigEndian, uint64(length)); err != nil {
			return err
		}
	}
	if _, err := c.w.Write(payload); err != nil {
		return err
	}
	return c.w.Flush()
}

func (c *wsServerConn) close() {
	_ = c.conn.Close()
}

func computeAccept(key string) string {
	const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	sum := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}
