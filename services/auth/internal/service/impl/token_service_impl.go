package impl

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"auth/internal/domain"
	"auth/internal/dto"
	"auth/internal/netutil"
	"auth/internal/observability/metrics"
	"auth/internal/observability/middleware"
	"auth/internal/store"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ====== Config ======

type TokenConfig struct {
	Issuer     string        // e.g. "e2ee-auth"
	Audience   string        // e.g. "e2ee-clients"
	AccessTTL  time.Duration // e.g. 15 * time.Minute
	RefreshTTL time.Duration // e.g. 30 * 24h
	SigningKey []byte        // HS256 secret
}

// ====== Claims ======

type AccessClaims struct {
	SID   string  `json:"sid"`           // session id
	DID   *string `json:"did,omitempty"` // device id (optional)
	Scope string  `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	SID                  string `json:"sid"` // session id
	jwt.RegisteredClaims        // jti == refresh_id
}

// ====== Service ======

type TokenServiceImpl struct {
	cfg   TokenConfig
	store *store.Store
}

func NewTokenServiceHS256(cfg TokenConfig, st *store.Store) *TokenServiceImpl {
	return &TokenServiceImpl{cfg: cfg, store: st}
}

// Issue creates a Session row (with a fresh RefreshID) and returns access+refresh tokens.
func (t *TokenServiceImpl) Issue(
	ctx context.Context,
	user *domain.User,
	deviceID *domain.DeviceID,
	ip, ua string,
) (*dto.TokenResponse, error) {
	result := "success"
	defer func() {
		metrics.TokensIssuedTotal.WithLabelValues("issue", result).Inc()
	}()
	ip = normalizeIP(ip)
	ua = netutil.TruncateUserAgent(ua)
	now := time.Now().UTC()

	// 1) create session
	sess := &domain.Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		DeviceID:  (*uuid.UUID)(deviceID), // nil safe
		RefreshID: uuid.New(),
		ExpiresAt: now.Add(t.cfg.RefreshTTL),
		RevokedAt: nil,
		CreatedAt: now,
		IP:        ip,
		UserAgent: ua,
	}
	if err := t.store.Sessions().Create(ctx, sess); err != nil {
		result = "failure"
		return nil, err
	}

	// 2) sign access + refresh
	access, err := t.signAccess(user.ID, sess, now)
	if err != nil {
		result = "failure"
		return nil, err
	}
	refresh, err := t.signRefresh(user.ID, sess, now)
	if err != nil {
		result = "failure"
		return nil, err
	}

	reqID := middleware.RequestIDFromContext(ctx)
	traceID := middleware.TraceIDFromContext(ctx)
	slog.Info("issued tokens", "session_id", sess.ID, "user_id", user.ID, "device_id", deviceID, "request_id", reqID, "trace_id", traceID)

	return &dto.TokenResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(t.cfg.AccessTTL.Seconds()),
	}, nil
}

// Refresh validates the refresh JWT, checks session state, rotates refresh id, and returns new tokens.
func (t *TokenServiceImpl) Refresh(ctx context.Context, refreshToken string, ip, ua string) (*dto.TokenResponse, error) {
	result := "success"
	defer func() {
		metrics.TokensIssuedTotal.WithLabelValues("refresh", result).Inc()
	}()
	ip = normalizeIP(ip)
	ua = netutil.TruncateUserAgent(ua)
	now := time.Now().UTC()

	// 1) parse & validate refresh JWT
	parsed, claims, err := t.parseRefresh(refreshToken)
	if err != nil || !parsed.Valid {
		result = "failure"
		return nil, errors.New("invalid token")
	}

	// 2) lookup session by refresh_id (claims.ID) and validate state
	sess, err := t.store.Sessions().GetByRefreshID(ctx, uuid.MustParse(claims.ID))
	if err != nil {
		result = "failure"
		return nil, errors.New("invalid token")
	}
	if sess.RevokedAt != nil || now.After(sess.ExpiresAt) {
		result = "failure"
		return nil, errors.New("session expired or revoked")
	}

	// (Optional) you can verify the IP/UA drift here if you want to bind sessions tightly.

	// 3) rotate refresh id + extend session expiry
	newRID := uuid.New()
	newExp := now.Add(t.cfg.RefreshTTL)
	if err := t.store.Sessions().Rotate(ctx, sess.ID, newRID, newExp, ip, ua); err != nil {
		result = "failure"
		return nil, err
	}
	sess.RefreshID = newRID
	sess.ExpiresAt = newExp
	sess.IP = ip
	sess.UserAgent = ua

	// 4) mint new access+refresh
	accessJWT, err := t.signAccess(sess.UserID, sess, now)
	if err != nil {
		result = "failure"
		return nil, err
	}
	refreshJWT, err := t.signRefresh(sess.UserID, sess, now)
	if err != nil {
		result = "failure"
		return nil, err
	}

	reqID := middleware.RequestIDFromContext(ctx)
	traceID := middleware.TraceIDFromContext(ctx)
	slog.Info("refreshed tokens", "session_id", sess.ID, "user_id", sess.UserID, "request_id", reqID, "trace_id", traceID)

	return &dto.TokenResponse{
		AccessToken:  accessJWT,
		RefreshToken: refreshJWT,
		ExpiresIn:    int64(t.cfg.AccessTTL.Seconds()),
	}, nil
}

func (t *TokenServiceImpl) RevokeSession(ctx context.Context, sessionID domain.SessionID) error {
	return t.store.Sessions().Revoke(ctx, uuid.UUID(sessionID), time.Now().UTC())
}

// ====== Helpers ======

func (t *TokenServiceImpl) signAccess(userID uuid.UUID, sess *domain.Session, now time.Time) (string, error) {
	jti := uuid.New().String() // unique per access token
	var did *string
	if sess.DeviceID != nil {
		s := sess.DeviceID.String()
		did = &s
	}
	claims := AccessClaims{
		SID:   sess.ID.String(),
		DID:   did,
		Scope: "user", // customize if you add scopes
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.cfg.Issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{t.cfg.Audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(t.cfg.AccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        jti,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.cfg.SigningKey)
}

func (t *TokenServiceImpl) signRefresh(userID uuid.UUID, sess *domain.Session, now time.Time) (string, error) {
	claims := RefreshClaims{
		SID: sess.ID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    t.cfg.Issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{t.cfg.Audience},
			ExpiresAt: jwt.NewNumericDate(sess.ExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        sess.RefreshID.String(), // <-- bind JWT to DB session row
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(t.cfg.SigningKey)
}

func (t *TokenServiceImpl) parseRefresh(tokenStr string) (*jwt.Token, *RefreshClaims, error) {
	claims := &RefreshClaims{}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	tok, err := parser.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
		return t.cfg.SigningKey, nil
	})
	if err != nil {
		return nil, nil, err
	}
	// Optional: enforce issuer/audience manually (kept explicit for clarity)
	if claims.Issuer != t.cfg.Issuer {
		return nil, nil, errors.New("bad issuer")
	}
	if !containsAudience(claims.Audience, t.cfg.Audience) {
		return nil, nil, errors.New("bad audience")
	}
	return tok, claims, nil
}

// containsAudience checks if the expected audience is present in the claim audience list.
func containsAudience(aud jwt.ClaimStrings, expected string) bool {
	for _, a := range aud {
		if a == expected {
			return true
		}
	}
	return false
}

func normalizeIP(ip string) string {
	if normalized, ok := netutil.NormalizeIP(ip); ok {
		return normalized
	}
	return strings.TrimSpace(ip)
}
