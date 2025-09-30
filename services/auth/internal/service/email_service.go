package service

import "context"

type EmailService interface {
	SendVerification(ctx context.Context, to string, token string) error
	SendMfaSetup(ctx context.Context, to string, otpURI string) error
}
