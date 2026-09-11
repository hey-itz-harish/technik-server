package mail

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"technik-server/config"
	"technik-server/logger"
)

type ZohoMailService struct {
	cfg *config.Config
}

func NewZohoMailService(cfg *config.Config) *ZohoMailService {
	return &ZohoMailService{cfg: cfg}
}

// SendActivationEmail sends an account activation link via Zoho Mail SMTP.
// activationLink is the primary CTA (points at the frontend SPA, valid ~24h).
// resendLink is a secondary link (points at the backend directly, valid ~7 days)
// that lets the recipient get a fresh activation link even after the primary
// one has expired.
func (z *ZohoMailService) SendActivationEmail(toEmail, recipientName, activationLink, resendLink string) error {
	if z.cfg.ZohoUser == "" || z.cfg.ZohoPass == "" {
		logger.Error("Zoho SMTP credentials not configured in .env (ZOHO_SMTP_USER & ZOHO_SMTP_PASS required)")
		logger.Info("--------------------------------------------------")
		logger.Info("[DEV MOCK ACTIVATION LINK] To: %s (%s) | Link: %s | Resend Link: %s", toEmail, recipientName, activationLink, resendLink)
		logger.Info("--------------------------------------------------")
		return nil
	}

	from := z.cfg.ZohoFrom
	if from == "" {
		from = z.cfg.ZohoUser
	}

	subject := "Subject: Technik Olympiad - Activate Your Account\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\nFrom: Technik Olympiad <" + from + ">\nTo: " + toEmail + "\n\n"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; background-color: #f8fafc; margin: 0; padding: 20px;">
  <div style="max-width: 520px; margin: 0 auto; background: #ffffff; padding: 35px; border-radius: 16px; border: 1px solid #e2e8f0; box-shadow: 0 4px 20px rgba(0,0,0,0.05);">
    <div style="text-align: center; margin-bottom: 25px;">
      <h2 style="color: #0c1e45; margin: 0; font-size: 24px; font-weight: 800;">TECHNIK OLYMPIAD</h2>
      <p style="color: #64748b; font-size: 13px; margin-top: 4px;">Empowering Young Minds</p>
    </div>
    <p style="color: #334155; font-size: 15px;">Dear <strong>%s</strong>,</p>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">
      Thank you for registering with Technik Olympiad. Please click the button below to activate your account:
    </p>
    <div style="text-align: center; margin: 30px 0;">
      <a href="%s" style="background: linear-gradient(135deg, #0c1e45 0%%, #2563eb 100%%); color: #ffffff; font-size: 15px; font-weight: bold; text-decoration: none; padding: 14px 28px; border-radius: 10px; display: inline-block;">
        Activate Account Now
      </a>
    </div>
    <p style="color: #64748b; font-size: 13px; line-height: 1.5;">
      Alternatively, copy and paste this link into your browser:<br>
      <a href="%s" style="color: #2563eb; word-break: break-all;">%s</a>
    </p>
    <p style="color: #94a3b8; font-size: 12px; margin-top: 20px;">
      ⏰ This activation link is valid for <strong>24 hours</strong>. Without activation, login and MFA setup will remain disabled.
    </p>
    <hr style="border: none; border-top: 1px solid #f1f5f9; margin: 25px 0;">
    <p style="color: #64748b; font-size: 12px; line-height: 1.5; text-align: center;">
      Link expired? <a href="%s" style="color: #ea580c; font-weight: bold;">Click here to resend the activation email</a>.
    </p>
    <hr style="border: none; border-top: 1px solid #f1f5f9; margin: 25px 0;">
    <p style="font-size: 12px; color: #94a3b8; text-align: center; margin: 0;">
      © 2026 Technik Olympiad Private Limited. All rights reserved.
    </p>
  </div>
</body>
</html>
`, recipientName, activationLink, activationLink, activationLink, resendLink)

	return z.sendRawMail(toEmail, from, subject+mime+body)
}

// SendOTPEmail sends a styled HTML OTP email via Zoho Mail SMTP
func (z *ZohoMailService) SendOTPEmail(toEmail, recipientName, otpCode string) error {
	if z.cfg.ZohoUser == "" || z.cfg.ZohoPass == "" {
		logger.Error("Zoho SMTP credentials not configured in .env (ZOHO_SMTP_USER & ZOHO_SMTP_PASS required)")
		logger.Info("--------------------------------------------------")
		logger.Info("[DEV MOCK OTP EMAIL] To: %s (%s) | OTP: %s", toEmail, recipientName, otpCode)
		logger.Info("--------------------------------------------------")
		return nil
	}

	from := z.cfg.ZohoFrom
	if from == "" {
		from = z.cfg.ZohoUser
	}

	subject := "Subject: Technik Olympiad - Your OTP Verification Code\n"
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\nFrom: Technik Olympiad <" + from + ">\nTo: " + toEmail + "\n\n"
	body := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; background-color: #f8fafc; margin: 0; padding: 20px;">
  <div style="max-width: 520px; margin: 0 auto; background: #ffffff; padding: 35px; border-radius: 16px; border: 1px solid #e2e8f0; box-shadow: 0 4px 20px rgba(0,0,0,0.05);">
    <div style="text-align: center; margin-bottom: 25px;">
      <h2 style="color: #0c1e45; margin: 0; font-size: 24px; font-weight: 800;">TECHNIK OLYMPIAD</h2>
      <p style="color: #64748b; font-size: 13px; margin-top: 4px;">Empowering Young Minds</p>
    </div>
    <p style="color: #334155; font-size: 15px;">Dear <strong>%s</strong>,</p>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">
      Your One-Time Password (OTP) for account verification and security authentication is:
    </p>
    <div style="background: linear-gradient(135deg, #fff7ed 0%%, #ffedd5 100%%); color: #ea580c; font-size: 34px; font-weight: 900; text-align: center; letter-spacing: 8px; padding: 18px; border-radius: 12px; border: 1px solid #fed7aa; margin: 25px 0;">
      %s
    </div>
    <p style="color: #64748b; font-size: 13px; line-height: 1.5;">
      ⏰ This OTP code is valid for <strong>10 minutes</strong>. Do not share this code with anyone.
    </p>
    <hr style="border: none; border-top: 1px solid #f1f5f9; margin: 25px 0;">
    <p style="font-size: 12px; color: #94a3b8; text-align: center; margin: 0;">
      © 2026 Technik Olympiad Private Limited. All rights reserved.
    </p>
  </div>
</body>
</html>
`, recipientName, otpCode)

	return z.sendRawMail(toEmail, from, subject+mime+body)
}

func (z *ZohoMailService) sendRawMail(toEmail, from, rawBody string) error {
	msg := []byte(rawBody)
	addr := fmt.Sprintf("%s:%s", z.cfg.ZohoHost, z.cfg.ZohoPort)
	auth := smtp.PlainAuth("", z.cfg.ZohoUser, z.cfg.ZohoPass, z.cfg.ZohoHost)

	var err error
	if z.cfg.ZohoPort == "465" {
		tlsconfig := &tls.Config{
			InsecureSkipVerify: false,
			ServerName:         z.cfg.ZohoHost,
		}
		conn, dialErr := tls.Dial("tcp", addr, tlsconfig)
		if dialErr != nil {
			logger.Error("Failed to connect to Zoho SSL SMTP: %v", dialErr)
			return dialErr
		}
		client, clientErr := smtp.NewClient(conn, z.cfg.ZohoHost)
		if clientErr != nil {
			logger.Error("Failed to create Zoho SMTP client: %v", clientErr)
			return clientErr
		}
		defer client.Close()

		if err = client.Auth(auth); err != nil {
			return err
		}
		if err = client.Mail(from); err != nil {
			return err
		}
		if err = client.Rcpt(toEmail); err != nil {
			return err
		}
		w, err := client.Data()
		if err != nil {
			return err
		}
		_, err = w.Write(msg)
		if err != nil {
			return err
		}
		err = w.Close()
		client.Quit()
	} else {
		host, _, _ := net.SplitHostPort(addr)
		auth := smtp.PlainAuth("", z.cfg.ZohoUser, z.cfg.ZohoPass, host)
		err = smtp.SendMail(addr, auth, from, []string{toEmail}, msg)
	}

	if err != nil {
		logger.Error("Failed to send email to %s via Zoho Mail: %v", toEmail, err)
		return err
	}

	logger.Info("Successfully sent email to %s via Zoho Mail SMTP", toEmail)
	return nil
}
