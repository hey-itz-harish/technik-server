package mail

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"

	"technik-server/config"
	"technik-server/logger"
)

type ZohoMailService struct {
	cfg           *config.Config
	resendService *ResendMailService
}

func NewZohoMailService(cfg *config.Config) *ZohoMailService {
	return &ZohoMailService{
		cfg:           cfg,
		resendService: NewResendMailService(cfg),
	}
}

// SendActivationEmail sends an account activation link via Resend API (HTTPS) or Zoho SMTP.
func (z *ZohoMailService) SendActivationEmail(toEmail, recipientName, activationLink, resendLink string) error {
	htmlBody := fmt.Sprintf(`
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

	logger.Info("[ACCOUNT ACTIVATION LINK] To: %s (%s) | Link: %s", toEmail, recipientName, activationLink)

	// 1. Primary: Use Resend HTTPS API if configured (Zero blocking on Railway)
	if z.cfg.ResendApiKey != "" {
		return z.resendService.SendEmail(toEmail, "Technik Olympiad - Activate Your Account", htmlBody)
	}

	// 2. Secondary: Fallback to Zoho SMTP if configured
	if z.cfg.ZohoUser == "" || z.cfg.ZohoPass == "" {
		logger.Error("Neither RESEND_API_KEY nor ZOHO_SMTP credentials are configured in .env")
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

	return z.sendRawMail(toEmail, from, subject+mime+htmlBody)
}

// SendOTPEmail sends a styled HTML OTP email via Resend API (HTTPS) or Zoho Mail SMTP
func (z *ZohoMailService) SendOTPEmail(toEmail, recipientName, otpCode string) error {
	htmlBody := fmt.Sprintf(`
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

	// 1. Primary: Use Resend HTTPS API if configured (Zero blocking on Railway)
	if z.cfg.ResendApiKey != "" {
		return z.resendService.SendEmail(toEmail, "Technik Olympiad - Your OTP Verification Code", htmlBody)
	}

	// 2. Secondary: Fallback to Zoho SMTP if configured
	if z.cfg.ZohoUser == "" || z.cfg.ZohoPass == "" {
		logger.Error("Neither RESEND_API_KEY nor ZOHO_SMTP credentials are configured in .env")
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

	return z.sendRawMail(toEmail, from, subject+mime+htmlBody)
}

func (z *ZohoMailService) sendRawMail(toEmail, from, rawBody string) error {
	msg := []byte(rawBody)
	host := z.cfg.ZohoHost
	port := z.cfg.ZohoPort
	user := z.cfg.ZohoUser
	pass := z.cfg.ZohoPass

	var err error
	if port == "465" {
		err = z.sendSSLMail(host, port, user, pass, from, toEmail, msg)
		if err != nil {
			logger.Error("Zoho SSL SMTP (port 465) failed: %v. Retrying via STARTTLS (port 587)...", err)
			err = z.sendSTARTTLSMail(host, "587", user, pass, from, toEmail, msg)
			if err == nil {
				port = "587 (fallback)"
			}
		}
	} else {
		// Port 587 or default: try 587 with fast timeout, fallback to SSL 465 if 587 is blocked
		err = z.sendSTARTTLSMail(host, port, user, pass, from, toEmail, msg)
		if err != nil {
			logger.Error("Zoho STARTTLS (port %s) failed: %v. Retrying via SSL (port 465)...", port, err)
			err = z.sendSSLMail(host, "465", user, pass, from, toEmail, msg)
			if err == nil {
				port = "465 (fallback)"
			}
		}
	}

	if err != nil {
		logger.Error("Failed to send email to %s via Zoho Mail: %v", toEmail, err)
		return err
	}

	logger.Info("Successfully sent email to %s via Zoho Mail SMTP (Port %s)", toEmail, port)
	return nil
}

func (z *ZohoMailService) sendSSLMail(host, port, user, pass, from, toEmail string, msg []byte) error {
	addr := net.JoinHostPort(host, port)
	auth := smtp.PlainAuth("", user, pass, host)
	tlsconfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         host,
	}
	dialer := &net.Dialer{
		Timeout: 7 * time.Second,
	}
	conn, dialErr := tls.DialWithDialer(dialer, "tcp", addr, tlsconfig)
	if dialErr != nil {
		return dialErr
	}
	client, clientErr := smtp.NewClient(conn, host)
	if clientErr != nil {
		conn.Close()
		return clientErr
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(toEmail); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

func (z *ZohoMailService) sendSTARTTLSMail(host, port, user, pass, from, toEmail string, msg []byte) error {
	addr := net.JoinHostPort(host, port)
	conn, err := net.DialTimeout("tcp", addr, 7*time.Second)
	if err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return err
	}
	defer client.Close()

	tlsconfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         host,
	}
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsconfig); err != nil {
			return err
		}
	}
	auth := smtp.PlainAuth("", user, pass, host)
	if ok, _ := client.Extension("AUTH"); ok {
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(from); err != nil {
		return err
	}
	if err := client.Rcpt(toEmail); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
