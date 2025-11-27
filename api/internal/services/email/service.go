package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mooncorn/gshub/api/config"
)

type Service struct {
	config *config.Config
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		config: cfg,
	}
}

// SendVerificationEmail sends an email verification link
func (s *Service) SendVerificationEmail(to, token string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.config.FrontendURL, token)

	subject := "Verify your email - GSHUB.PRO"
	htmlContent := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<meta charset="utf-8">
		</head>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
				<h1 style="color: #4F46E5;">Welcome to GSHUB.PRO!</h1>
				<p>Thank you for creating an account. Please verify your email address by clicking the link below:</p>
				<p style="margin: 30px 0;">
					<a href="%s" style="background-color: #4F46E5; color: white; padding: 12px 24px; text-decoration: none; border-radius: 5px; display: inline-block;">
						Verify Email Address
					</a>
				</p>
				<p style="color: #666; font-size: 14px;">
					If you didn't create this account, you can safely ignore this email.
				</p>
				<p style="color: #666; font-size: 14px;">
					This link will expire in 24 hours.
				</p>
			</div>
		</body>
		</html>
	`, verifyURL)

	plainContent := fmt.Sprintf(`
Welcome to GSHUB.PRO!

Thank you for creating an account. Please verify your email address by visiting:

%s

If you didn't create this account, you can safely ignore this email.

This link will expire in 24 hours.
	`, verifyURL)

	return s.sendEmail(to, subject, plainContent, htmlContent)
}

// SendPasswordResetEmail sends a password reset link
func (s *Service) SendPasswordResetEmail(to, token string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.config.FrontendURL, token)

	subject := "Reset your password - GSHUB.PRO"
	htmlContent := fmt.Sprintf(`
		<!DOCTYPE html>
		<html>
		<head>
			<meta charset="utf-8">
		</head>
		<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333;">
			<div style="max-width: 600px; margin: 0 auto; padding: 20px;">
				<h1 style="color: #4F46E5;">Password Reset Request</h1>
				<p>We received a request to reset your password. Click the link below to create a new password:</p>
				<p style="margin: 30px 0;">
					<a href="%s" style="background-color: #4F46E5; color: white; padding: 12px 24px; text-decoration: none; border-radius: 5px; display: inline-block;">
						Reset Password
					</a>
				</p>
				<p style="color: #666; font-size: 14px;">
					If you didn't request a password reset, you can safely ignore this email. Your password will not be changed.
				</p>
				<p style="color: #666; font-size: 14px;">
					This link will expire in 1 hour.
				</p>
			</div>
		</body>
		</html>
	`, resetURL)

	plainContent := fmt.Sprintf(`
Password Reset Request

We received a request to reset your password. Visit the link below to create a new password:

%s

If you didn't request a password reset, you can safely ignore this email. Your password will not be changed.

This link will expire in 1 hour.
	`, resetURL)

	return s.sendEmail(to, subject, plainContent, htmlContent)
}

// MailerSendRequest represents the MailerSend API request structure
type MailerSendRequest struct {
	From    EmailAddress   `json:"from"`
	To      []EmailAddress `json:"to"`
	Subject string         `json:"subject"`
	Text    string         `json:"text"`
	HTML    string         `json:"html"`
}

type EmailAddress struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

// sendEmail sends an email using MailerSend
func (s *Service) sendEmail(to, subject, plainContent, htmlContent string) error {
	// If no API key is configured, log the email instead (for development)
	if s.config.MailerSendAPIKey == "" {
		fmt.Printf("\n=== EMAIL (MailerSend not configured) ===\n")
		fmt.Printf("To: %s\n", to)
		fmt.Printf("Subject: %s\n", subject)
		fmt.Printf("Content:\n%s\n", plainContent)
		fmt.Printf("=====================================\n\n")
		return nil
	}

	// Prepare request payload
	payload := MailerSendRequest{
		From: EmailAddress{
			Email: s.config.MailerSendFromEmail,
			Name:  s.config.MailerSendFromName,
		},
		To: []EmailAddress{
			{Email: to},
		},
		Subject: subject,
		Text:    plainContent,
		HTML:    htmlContent,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://api.mailersend.com/v1/email", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.config.MailerSendAPIKey))

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode >= 400 {
		var errorBody bytes.Buffer
		errorBody.ReadFrom(resp.Body)
		return fmt.Errorf("mailersend returned error: %d - %s", resp.StatusCode, errorBody.String())
	}

	return nil
}
