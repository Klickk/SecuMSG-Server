package impl

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"

	"golang.org/x/crypto/argon2"
)

type Argon2Params struct {
	// Stored alongside the hash so verification uses the original cost.
	Time    uint32 `json:"t"` // iterations
	Memory  uint32 `json:"m"` // KiB (e.g., 64*1024 = 64MB)
	Threads uint8  `json:"p"` // parallelism
	KeyLen  uint32 `json:"k"` // bytes (e.g., 32)
	SaltLen uint32 `json:"s"` // bytes (e.g., 16)
}

type PasswordServiceImpl struct {
	currentVer int          // bump when you change policy
	cur        Argon2Params // current policy used for new hashes
	algoName   string       // "argon2id"
}

func NewPasswordServiceArgon2id() *PasswordServiceImpl {
	return &PasswordServiceImpl{
		currentVer: 1,
		algoName:   "argon2id",
		cur: Argon2Params{
			Time:    3,
			Memory:  64 * 1024, // 64 MiB
			Threads: 1,
			KeyLen:  32,
			SaltLen: 16,
		},
	}
}

func (p *PasswordServiceImpl) Hash(password string) (hash, salt, paramsJSON []byte, algo string, ver int, err error) {
	if password == "" {
		return nil, nil, nil, "", 0, ErrEmptyPassword
	}
	salt = make([]byte, p.cur.SaltLen)
	if _, err = rand.Read(salt); err != nil {
		return nil, nil, nil, "", 0, err
	}
	// derive key
	hash = argon2.IDKey([]byte(password), salt, p.cur.Time, p.cur.Memory, p.cur.Threads, p.cur.KeyLen)
	paramsJSON, err = json.Marshal(p.cur)
	if err != nil {
		return nil, nil, nil, "", 0, err
	}
	return hash, salt, paramsJSON, p.algoName, p.currentVer, nil
}

func (p *PasswordServiceImpl) Verify(password string, cred interface {
	GetAlgo() string
	GetHash() []byte
	GetSalt() []byte
	GetParamsJSON() []byte
	GetPasswordVer() int
}) (rehashNeeded bool, ok bool) {
	if cred.GetAlgo() != p.algoName {
		return true, false // different algorithm, request rehash on success	
	}
	var stored Argon2Params
	if err := json.Unmarshal(cred.GetParamsJSON(), &stored); err != nil {
		return true, false
	}
	calculated := argon2.IDKey([]byte(password), cred.GetSalt(), stored.Time, stored.Memory, stored.Threads, stored.KeyLen)
	ok = subtle.ConstantTimeCompare(calculated, cred.GetHash()) == 1

	// Rehash if policy changed (params or version)
	rehashNeeded = ok && (cred.GetPasswordVer() != p.currentVer ||
		stored.Time != p.cur.Time ||
		stored.Memory != p.cur.Memory ||
		stored.Threads != p.cur.Threads ||
		stored.KeyLen != p.cur.KeyLen ||
		stored.SaltLen != p.cur.SaltLen)

	return rehashNeeded, ok
}