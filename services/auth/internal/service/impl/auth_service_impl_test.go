package impl

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"auth/internal/domain"
	"auth/internal/dto"

	"github.com/google/uuid"
)

type stubPasswordService struct {
	hashFunc   func(password string) (hash, salt, paramsJSON []byte, algo string, ver int, err error)
	verifyFunc func(password string, cred interface {
		GetAlgo() string
		GetHash() []byte
		GetSalt() []byte
		GetParamsJSON() []byte
		GetPasswordVer() int
	}) (rehashNeeded bool, ok bool)

	hashCalls   []string
	verifyCalls []struct {
		password string
		algo     string
		hash     []byte
	}
}

func (s *stubPasswordService) Hash(password string) (hash, salt, paramsJSON []byte, algo string, ver int, err error) {
	s.hashCalls = append(s.hashCalls, password)
	if s.hashFunc != nil {
		return s.hashFunc(password)
	}
	return nil, nil, nil, "", 0, nil
}

func (s *stubPasswordService) Verify(password string, cred interface {
	GetAlgo() string
	GetHash() []byte
	GetSalt() []byte
	GetParamsJSON() []byte
	GetPasswordVer() int
},
) (rehashNeeded bool, ok bool) {
	s.verifyCalls = append(s.verifyCalls, struct {
		password string
		algo     string
		hash     []byte
	}{password: password, algo: cred.GetAlgo(), hash: append([]byte(nil), cred.GetHash()...)})
	if s.verifyFunc != nil {
		return s.verifyFunc(password, cred)
	}
	return false, false
}

type stubTokenService struct {
	issueResponse *dto.TokenResponse
	issueErr      error

	issueCalls []struct {
		userID uuid.UUID
		ip     string
		ua     string
	}
}

func (s *stubTokenService) Issue(ctx context.Context, user *domain.User, deviceID *domain.DeviceID, ip, ua string) (*dto.TokenResponse, error) {
	s.issueCalls = append(s.issueCalls, struct {
		userID uuid.UUID
		ip     string
		ua     string
	}{userID: user.ID, ip: ip, ua: ua})
	if s.issueErr != nil {
		return nil, s.issueErr
	}
	return s.issueResponse, nil
}

func (s *stubTokenService) Refresh(ctx context.Context, refreshToken string, ip, ua string) (*dto.TokenResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubTokenService) RevokeSession(ctx context.Context, sessionID domain.SessionID) error {
	return errors.New("not implemented")
}

func (s *stubTokenService) VerifyAccess(ctx context.Context, req dto.VerifyRequest) (dto.VerifyResponse, error) {
	return dto.VerifyResponse{Valid: false}, errors.New("not implemented")
}

type memoryStore struct {
	mu          sync.Mutex
	users       map[uuid.UUID]*domain.User
	emailIndex  map[string]uuid.UUID
	usernameIdx map[string]uuid.UUID
	credentials map[uuid.UUID]*domain.PasswordCredential
}

type storeSnapshot struct {
	users       map[uuid.UUID]*domain.User
	emailIndex  map[string]uuid.UUID
	usernameIdx map[string]uuid.UUID
	credentials map[uuid.UUID]*domain.PasswordCredential
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		users:       make(map[uuid.UUID]*domain.User),
		emailIndex:  make(map[string]uuid.UUID),
		usernameIdx: make(map[string]uuid.UUID),
		credentials: make(map[uuid.UUID]*domain.PasswordCredential),
	}
}

func (m *memoryStore) WithTx(ctx context.Context, fn func(tx storeTx) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot := m.snapshot()
	tx := &memoryTx{store: m}
	if err := fn(tx); err != nil {
		m.restore(snapshot)
		return err
	}
	return nil
}

func (m *memoryStore) snapshot() storeSnapshot {
	users := make(map[uuid.UUID]*domain.User, len(m.users))
	for id, user := range m.users {
		copy := *user
		users[id] = &copy
	}
	creds := make(map[uuid.UUID]*domain.PasswordCredential, len(m.credentials))
	for id, cred := range m.credentials {
		copy := *cred
		creds[id] = &copy
	}
	emails := make(map[string]uuid.UUID, len(m.emailIndex))
	for k, v := range m.emailIndex {
		emails[k] = v
	}
	usernames := make(map[string]uuid.UUID, len(m.usernameIdx))
	for k, v := range m.usernameIdx {
		usernames[k] = v
	}
	return storeSnapshot{
		users:       users,
		emailIndex:  emails,
		usernameIdx: usernames,
		credentials: creds,
	}
}

func (m *memoryStore) restore(s storeSnapshot) {
	m.users = s.users
	m.emailIndex = s.emailIndex
	m.usernameIdx = s.usernameIdx
	m.credentials = s.credentials
}

func (m *memoryStore) userByEmail(email string) (*domain.User, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.emailIndex[email]
	if !ok {
		return nil, false
	}
	user := *m.users[id]
	return &user, true
}

func (m *memoryStore) credentialByUserID(userID uuid.UUID) (*domain.PasswordCredential, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cred, ok := m.credentials[userID]
	if !ok {
		return nil, false
	}
	copy := *cred
	return &copy, true
}

type memoryTx struct {
	store *memoryStore
}

func (m *memoryTx) Users() userStore { return &memoryUserStore{store: m.store} }

func (m *memoryTx) Credentials() credentialStore { return &memoryCredentialStore{store: m.store} }

type memoryUserStore struct {
	store *memoryStore
}

func (u *memoryUserStore) Create(ctx context.Context, usr *domain.User) error {
	copy := *usr
	u.store.users[usr.ID] = &copy
	u.store.emailIndex[usr.Email] = usr.ID
	u.store.usernameIdx[usr.Username] = usr.ID
	return nil
}

func (u *memoryUserStore) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	id, ok := u.store.emailIndex[email]
	if !ok {
		return nil, errors.New("user not found")
	}
	copy := *u.store.users[id]
	return &copy, nil
}

func (u *memoryUserStore) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	id, ok := u.store.usernameIdx[username]
	if !ok {
		return nil, errors.New("user not found")
	}
	copy := *u.store.users[id]
	return &copy, nil
}

type memoryCredentialStore struct {
	store *memoryStore
}

func (c *memoryCredentialStore) UpsertPassword(ctx context.Context, cred *domain.PasswordCredential) error {
	copy := *cred
	c.store.credentials[cred.UserID] = &copy
	return nil
}

func (c *memoryCredentialStore) GetPasswordByUserID(ctx context.Context, userID uuid.UUID) (*domain.PasswordCredential, error) {
	cred, ok := c.store.credentials[userID]
	if !ok {
		return nil, errors.New("credential not found")
	}
	return cred, nil
}

func TestAuthServiceRegisterCreatesUserAndPasswordCredential(t *testing.T) {
	store := newMemoryStore()
	ps := &stubPasswordService{
		hashFunc: func(password string) (hash, salt, paramsJSON []byte, algo string, ver int, err error) {
			return []byte("hash"), []byte("salt"), []byte("params"), "argon2id", 1, nil
		},
	}
	svc := &AuthServiceImpl{Store: store, PasswordService: ps}

	ctx := context.Background()
	req := dto.RegisterRequest{Email: "alice@example.com", Username: "alice", Password: "hunter22"}
	resp, err := svc.Register(ctx, req, "127.0.0.1", "unit-test")
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	if resp == nil || resp.UserID == "" {
		t.Fatalf("expected response with user id, got %+v", resp)
	}
	if !resp.RequiresEmailVerification {
		t.Fatalf("expected email verification to be required")
	}
	if len(ps.hashCalls) != 1 || ps.hashCalls[0] != req.Password {
		t.Fatalf("expected password hash to be invoked once with provided password")
	}

	user, ok := store.userByEmail(req.Email)
	if !ok {
		t.Fatalf("user was not persisted")
	}
	if user.Username != req.Username {
		t.Fatalf("stored username mismatch: got %q want %q", user.Username, req.Username)
	}

	cred, ok := store.credentialByUserID(uuid.MustParse(resp.UserID))
	if !ok {
		t.Fatalf("password credential was not stored")
	}
	if string(cred.Hash) != "hash" || string(cred.Salt) != "salt" || string(cred.ParamsJSON) != "params" {
		t.Fatalf("unexpected credential data: %+v", cred)
	}
	if cred.PasswordVer != 1 {
		t.Fatalf("unexpected password version: %d", cred.PasswordVer)
	}
}

func TestAuthServiceRegisterValidations(t *testing.T) {
	svc := &AuthServiceImpl{Store: newMemoryStore(), PasswordService: &stubPasswordService{}}
	ctx := context.Background()

	cases := []struct {
		name string
		req  dto.RegisterRequest
		want error
	}{
		{name: "missing email", req: dto.RegisterRequest{Username: "alice", Password: "hunter22"}, want: ErrEmptyCredential},
		{name: "missing username", req: dto.RegisterRequest{Email: "alice@example.com", Password: "hunter22"}, want: ErrEmptyCredential},
		{name: "short password", req: dto.RegisterRequest{Email: "alice@example.com", Username: "alice", Password: "short"}, want: ErrPasswordLength},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.Register(ctx, tc.req, "", ""); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestAuthServiceLoginWithEmailSuccess(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()

	now := time.Now().UTC()
	user := &domain.User{
		ID:        uuid.New(),
		Email:     "bob@example.com",
		Username:  "bob",
		CreatedAt: now,
		UpdatedAt: now,
	}
	cred := &domain.PasswordCredential{
		ID:          uuid.New(),
		UserID:      user.ID,
		Algo:        "argon2id",
		Hash:        []byte("stored-hash"),
		Salt:        []byte("stored-salt"),
		ParamsJSON:  []byte("stored-params"),
		PasswordVer: 1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.WithTx(ctx, func(tx storeTx) error {
		if err := tx.Users().Create(ctx, user); err != nil {
			return err
		}
		return tx.Credentials().UpsertPassword(ctx, cred)
	}); err != nil {
		t.Fatalf("failed to seed store: %v", err)
	}

	ps := &stubPasswordService{
		verifyFunc: func(password string, cred interface {
			GetAlgo() string
			GetHash() []byte
			GetSalt() []byte
			GetParamsJSON() []byte
			GetPasswordVer() int
		},
		) (bool, bool) {
			if password != "super-secret" {
				t.Fatalf("unexpected password in verify: %q", password)
			}
			return false, true
		},
	}

	ts := &stubTokenService{issueResponse: &dto.TokenResponse{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 3600}}

	svc := &AuthServiceImpl{Store: store, PasswordService: ps, TService: ts}

	resp, err := svc.Login(ctx, dto.LoginRequest{EmailOrUsername: user.Email, Password: "super-secret"}, "10.0.0.1", "unit-test")
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}
	if resp == nil || resp.AccessToken != "access" || resp.RefreshToken != "refresh" {
		t.Fatalf("unexpected login response: %+v", resp)
	}
	if len(ps.hashCalls) != 0 {
		t.Fatalf("expected no rehash, got %d hash calls", len(ps.hashCalls))
	}
	if len(ps.verifyCalls) != 1 {
		t.Fatalf("expected verify to be called once, got %d", len(ps.verifyCalls))
	}
	if len(ts.issueCalls) != 1 || ts.issueCalls[0].userID != user.ID {
		t.Fatalf("token service issue not invoked correctly: %+v", ts.issueCalls)
	}
}

func TestAuthServiceLoginRehashesWhenNeeded(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()

	now := time.Now().UTC()
	user := &domain.User{
		ID:        uuid.New(),
		Email:     "carol@example.com",
		Username:  "carol",
		CreatedAt: now,
		UpdatedAt: now,
	}
	cred := &domain.PasswordCredential{
		ID:          uuid.New(),
		UserID:      user.ID,
		Algo:        "argon2id",
		Hash:        []byte("legacy-hash"),
		Salt:        []byte("legacy-salt"),
		ParamsJSON:  []byte("legacy-params"),
		PasswordVer: 1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := store.WithTx(ctx, func(tx storeTx) error {
		if err := tx.Users().Create(ctx, user); err != nil {
			return err
		}
		return tx.Credentials().UpsertPassword(ctx, cred)
	}); err != nil {
		t.Fatalf("failed to seed store: %v", err)
	}

	ps := &stubPasswordService{
		verifyFunc: func(password string, cred interface {
			GetAlgo() string
			GetHash() []byte
			GetSalt() []byte
			GetParamsJSON() []byte
			GetPasswordVer() int
		},
		) (bool, bool) {
			return true, true
		},
		hashFunc: func(password string) (hash, salt, paramsJSON []byte, algo string, ver int, err error) {
			if password != "updated-secret" {
				t.Fatalf("unexpected password to hash: %q", password)
			}
			return []byte("new-hash"), []byte("new-salt"), []byte("new-params"), "argon2id", 2, nil
		},
	}

	ts := &stubTokenService{issueResponse: &dto.TokenResponse{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 3600}}

	svc := &AuthServiceImpl{Store: store, PasswordService: ps, TService: ts}

	if _, err := svc.Login(ctx, dto.LoginRequest{EmailOrUsername: user.Username, Password: "updated-secret"}, "10.0.0.2", "unit-test"); err != nil {
		t.Fatalf("login returned error: %v", err)
	}

	if len(ps.hashCalls) != 1 {
		t.Fatalf("expected one rehash call, got %d", len(ps.hashCalls))
	}

	stored, ok := store.credentialByUserID(user.ID)
	if !ok {
		t.Fatalf("credential missing after rehash")
	}
	if string(stored.Hash) != "new-hash" || string(stored.Salt) != "new-salt" || string(stored.ParamsJSON) != "new-params" {
		t.Fatalf("credential was not updated: %+v", stored)
	}
	if stored.PasswordVer != 2 {
		t.Fatalf("expected password version 2, got %d", stored.PasswordVer)
	}
}
