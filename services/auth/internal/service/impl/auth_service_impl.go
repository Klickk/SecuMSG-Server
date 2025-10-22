package impl

import (
	"auth/internal/domain"
	"auth/internal/dto"
	"auth/internal/service"
	"auth/internal/store"
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type AuthServiceImpl struct {
	Store           dataStore
	PasswordService service.PasswordService
	TService        service.TokenService
}

func NewAuthServiceImpl(store *store.Store, passwordService service.PasswordService, tokenService service.TokenService) *AuthServiceImpl {
	return &AuthServiceImpl{
		Store:           gormStoreAdapter{store: store},
		PasswordService: passwordService,
		TService:        tokenService,
	}
}

type dataStore interface {
	WithTx(ctx context.Context, fn func(tx storeTx) error) error
}

type storeTx interface {
	Users() userStore
	Credentials() credentialStore
}

type userStore interface {
	Create(ctx context.Context, usr *domain.User) error
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByUsername(ctx context.Context, username string) (*domain.User, error)
}

type credentialStore interface {
	UpsertPassword(ctx context.Context, c *domain.PasswordCredential) error
	GetPasswordByUserID(ctx context.Context, userID uuid.UUID) (*domain.PasswordCredential, error)
}

type gormStoreAdapter struct {
	store *store.Store
}

func (g gormStoreAdapter) WithTx(ctx context.Context, fn func(tx storeTx) error) error {
	if g.store == nil {
		return errors.New("nil store")
	}
	return g.store.WithTx(ctx, func(tx *store.Store) error {
		return fn(gormTxAdapter{tx: tx})
	})
}

type gormTxAdapter struct {
	tx *store.Store
}

func (g gormTxAdapter) Users() userStore { return g.tx.Users() }

func (g gormTxAdapter) Credentials() credentialStore { return g.tx.Credentials() }

func (a *AuthServiceImpl) Register(ctx context.Context, r dto.RegisterRequest, ip, ua string) (*dto.RegisterResponse, error) {
	// 1) basic validation
	if r.Email == "" || r.Username == "" {
		return nil, ErrEmptyCredential
	}
	// (Optional) enforce min password length if password is used
	if r.Password != "" && len(r.Password) < 8 {
		return nil, ErrPasswordLength
	}

	var out dto.RegisterResponse

	// 2) single transaction: create user + (optional) password credential
	err := a.Store.WithTx(ctx, func(tx storeTx) error {
		now := time.Now().UTC()

		// 2a) create user
		u := &domain.User{
			ID:            uuid.New(),
			Email:         r.Email,
			Username:      r.Username,
			EmailVerified: false, // stays false until VerifyEmail succeeds
			IsDisabled:    false,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := tx.Users().Create(ctx, u); err != nil {
			return err // unique constraints bubble up (email/username)
		}

		// 2b) if password supplied, hash then persist credential
		if r.Password != "" {
			hash, salt, paramsJSON, algo, ver, err := a.PasswordService.Hash(r.Password)
			if err != nil {
				return err
			}
			cred := &domain.PasswordCredential{
				ID:          uuid.New(),
				UserID:      u.ID,
				Algo:        algo,
				Hash:        hash,
				Salt:        salt,
				ParamsJSON:  paramsJSON,
				PasswordVer: ver,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			if err := tx.Credentials().UpsertPassword(ctx, cred); err != nil {
				return err
			}
		}

		// (Optional) create email verification token + send email via a.Email
		// You’ll implement email_verifications store later.

		out = dto.RegisterResponse{
			UserID:                    u.ID.String(),
			RequiresEmailVerification: true,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &out, nil
}

// TO DO
func (s *AuthServiceImpl) VerifyEmail(ctx context.Context, token string) error {
	return nil
}

func (a *AuthServiceImpl) Login(ctx context.Context, r dto.LoginRequest, ip, ua string) (*dto.TokenResponse, error) {
	if r.EmailOrUsername == "" || r.Password == "" {
		return nil, ErrEmptyCredential
	}

	// We might need a tx if we rehash the password (write). Keep it simple: always use WithTx.
	var tokens *dto.TokenResponse

	err := a.Store.WithTx(ctx, func(tx storeTx) error {
		// 1) load user by email or username
		var user *domain.User
		var err error
		if looksLikeEmail(r.EmailOrUsername) {
			user, err = tx.Users().GetByEmail(ctx, r.EmailOrUsername)
		} else {
			user, err = tx.Users().GetByUsername(ctx, r.EmailOrUsername)
		}
		if err != nil {
			return domain.ErrInvalidCredentials // don't leak which field failed
		}
		if user.IsDisabled {
			return domain.ErrUserDisabled
		}
		// (Optional) enforce verified emails before login:
		// if !user.EmailVerified { return domain.ErrEmailNotVerified }

		// 2) load stored password credential
		cred, err := tx.Credentials().GetPasswordByUserID(ctx, user.ID)
		if err != nil {
			return domain.ErrInvalidCredentials
		}

		// 3) verify password (and decide if we should rehash)
		rehashNeeded, ok := a.PasswordService.Verify(r.Password, cred)
		if !ok {
			return domain.ErrInvalidCredentials
		}

		// 4) optional transparent rehash (policy upgrade)
		if rehashNeeded {
			newHash, newSalt, newParamsJSON, algo, ver, err := a.PasswordService.Hash(r.Password)
			if err != nil {
				return err
			}
			cred.Algo = algo
			cred.Hash = newHash
			cred.Salt = newSalt
			cred.ParamsJSON = newParamsJSON
			cred.PasswordVer = ver
			cred.UpdatedAt = time.Now().UTC()
			if err := tx.Credentials().UpsertPassword(ctx, cred); err != nil {
				return err
			}
		}

		// 5) mint tokens + persist session (TokenService handles session write)
		tr, err := a.TService.Issue(ctx, user, nil /*deviceID*/, ip, ua)
		if err != nil {
			return err
		}
		tokens = tr
		return nil
	})

	if err != nil {
		return nil, err
	}
	return tokens, nil
}

func looksLikeEmail(s string) bool { return strings.ContainsRune(s, '@') }

func (s *AuthServiceImpl) Logout(ctx context.Context, refreshToken string) error {
	return errors.New("not implemented")
}
