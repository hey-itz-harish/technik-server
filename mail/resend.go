package mail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"technik-server/config"
	"technik-server/logger"
)

type ResendMailService struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewResendMailService(cfg *config.Config) *ResendMailService {
	return &ResendMailService{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type resendEmailPayload struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Html    string   `json:"html"`
}

type resendResponse struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

func (r *ResendMailService) SendEmail(toEmail, subject, htmlBody string) error {
	if r.cfg.ResendApiKey == "" {
		return fmt.Errorf("RESEND_API_KEY is not configured")
	}

	from := r.cfg.ResendFrom
	if from == "" {
		from = "Technik Olympiad <onboarding@resend.dev>"
	}

	payload := resendEmailPayload{
		From:    from,
		To:      []string{toEmail},
		Subject: subject,
		Html:    htmlBody,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal email payload: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+r.cfg.ResendApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Resend API: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var resErr resendResponse
		_ = json.Unmarshal(respBody, &resErr)
		msg := resErr.Message
		if msg == "" {
			msg = string(respBody)
		}
		logger.Error("Resend API failed (%d) for %s: %s", resp.StatusCode, toEmail, msg)
		return fmt.Errorf("resend API error (%d): %s", resp.StatusCode, msg)
	}

	logger.Info("Successfully sent email to %s via Resend API (HTTPS Port 443)", toEmail)
	return nil
}
