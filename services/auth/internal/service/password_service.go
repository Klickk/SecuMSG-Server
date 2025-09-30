package service

type PasswordService interface {
	Hash(password string) (hash, salt, paramsJSON []byte, algo string, ver int, err error)
	Verify(password string, cred interface{ GetAlgo() string; GetHash() []byte; GetSalt() []byte; GetParamsJSON() []byte; GetPasswordVer() int }) (rehashNeeded bool, ok bool)
}
