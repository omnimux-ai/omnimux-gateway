package common

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"os"
	"slices"
	"strings"
	"time"
)

func generateMessageID() (string, error) {
	split := strings.Split(SMTPFrom, "@")
	if len(split) < 2 {
		return "", fmt.Errorf("invalid SMTP account")
	}
	domain := strings.Split(SMTPFrom, "@")[1]
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), GetRandomString(12), domain), nil
}

func shouldUseSMTPLoginAuth() bool {
	if SMTPForceAuthLogin {
		return true
	}
	return isOutlookServer(SMTPAccount) || slices.Contains(EmailLoginAuthServerList, SMTPServer)
}

func getSMTPAuth() smtp.Auth {
	return AutoSMTPAuth(SMTPAccount, SMTPToken)
}

func shouldAuthenticateSMTP() bool {
	return SMTPAccount != "" && SMTPToken != ""
}

func smtpTLSConfig() *tls.Config {
	return &tls.Config{
		ServerName:         SMTPServer,
		InsecureSkipVerify: SMTPInsecureSkipVerify, // #nosec G402 -- admin-controlled SMTP compatibility option.
	}
}

func newSMTPClient(addr string) (*smtp.Client, error) {
	if SMTPSSLEnabled || (SMTPPort == 465 && !SMTPStartTLSEnabled) {
		conn, err := tls.Dial("tcp", addr, smtpTLSConfig())
		if err != nil {
			return nil, err
		}
		client, err := smtp.NewClient(conn, SMTPServer)
		if err != nil {
			_ = conn.Close()
			return nil, err
		}
		return client, nil
	}

	client, err := smtp.Dial(addr)
	if err != nil {
		return nil, err
	}

	if SMTPStartTLSEnabled {
		startTLSSupported, _ := client.Extension("STARTTLS")
		if !startTLSSupported {
			_ = client.Close()
			return nil, fmt.Errorf("SMTP server does not support STARTTLS")
		}
		if err := client.StartTLS(smtpTLSConfig()); err != nil {
			_ = client.Close()
			return nil, err
		}
	}

	return client, nil
}

// isCloudflareEmailService reports whether SMTP settings point at Cloudflare Email Service.
func isCloudflareEmailService() bool {
	return SMTPAccount == "api_token" &&
		(SMTPServer == "smtp.mx.cloudflare.net" || SMTPServer == "" || strings.Contains(SMTPServer, "cloudflare"))
}

func cloudflareAccountID() string {
	if v := strings.TrimSpace(os.Getenv("CLOUDFLARE_ACCOUNT_ID")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("CF_ACCOUNT_ID"))
}

func cloudflareEmailAPIToken() string {
	if SMTPToken != "" {
		return SMTPToken
	}
	if v := strings.TrimSpace(os.Getenv("CLOUDFLARE_EMAIL_API_TOKEN")); v != "" {
		return v
	}
	return strings.TrimSpace(os.Getenv("CF_API_TOKEN"))
}

// sendEmailCloudflareWorker posts to a Cloudflare Worker that calls Email Sending REST
// from the CF edge (avoids datacenter IP blocks on the Email API).
func sendEmailCloudflareWorker(subject string, receiver string, content string) error {
	workerURL := strings.TrimSpace(os.Getenv("CLOUDFLARE_EMAIL_WORKER_URL"))
	secret := strings.TrimSpace(os.Getenv("CLOUDFLARE_EMAIL_RELAY_SECRET"))
	if workerURL == "" || secret == "" {
		return fmt.Errorf("CLOUDFLARE_EMAIL_WORKER_URL / CLOUDFLARE_EMAIL_RELAY_SECRET not set")
	}
	from := SMTPFrom
	if from == "" || !strings.Contains(from, "@") {
		return fmt.Errorf("SMTPFrom must be a full email address for Cloudflare Email Service")
	}
	payload := map[string]any{
		"to":      receiver,
		"from":    from,
		"subject": subject,
		"html":    content,
		"text":    stripHTMLTags(content),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, workerURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var parsed struct {
		Success bool `json:"success"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	if resp.StatusCode >= 300 || !parsed.Success {
		msg := strings.TrimSpace(string(respBody))
		if len(parsed.Errors) > 0 && parsed.Errors[0].Message != "" {
			msg = parsed.Errors[0].Message
		} else if parsed.Message != "" {
			msg = parsed.Message
		}
		return fmt.Errorf("cloudflare email worker failed (http %d): %s", resp.StatusCode, msg)
	}
	return nil
}

func sendEmailCloudflareREST(subject string, receiver string, content string) error {
	accountID := cloudflareAccountID()
	apiToken := cloudflareEmailAPIToken()
	if accountID == "" || apiToken == "" {
		return fmt.Errorf("Cloudflare Email Service requires CLOUDFLARE_ACCOUNT_ID and API token")
	}
	from := SMTPFrom
	if from == "" || !strings.Contains(from, "@") {
		return fmt.Errorf("SMTPFrom must be a full email address for Cloudflare Email Service")
	}
	payload := map[string]any{
		"to":      receiver,
		"from":    from,
		"subject": subject,
		"html":    content,
		"text":    stripHTMLTags(content),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/email/sending/send", accountID)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var parsed struct {
		Success bool `json:"success"`
		Errors  []struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(respBody, &parsed)
	if resp.StatusCode >= 300 || !parsed.Success {
		msg := strings.TrimSpace(string(respBody))
		if len(parsed.Errors) > 0 {
			msg = parsed.Errors[0].Message
		}
		return fmt.Errorf("cloudflare email send failed (http %d): %s", resp.StatusCode, msg)
	}
	return nil
}

func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// normalizeReceiverForCloudflareEmail rewrites destinations that Cloudflare Email
// Sending silently drops. Observed 2026-08: @gmail.com never appears in CF activity
// log, while the same mailbox via @googlemail.com is delivered (same Gmail inbox).
func normalizeReceiverForCloudflareEmail(receiver string) string {
	parts := strings.Split(receiver, ";")
	for i, p := range parts {
		addr := strings.TrimSpace(p)
		lower := strings.ToLower(addr)
		if strings.HasSuffix(lower, "@gmail.com") {
			local := addr[:len(addr)-len("@gmail.com")]
			parts[i] = local + "@googlemail.com"
			SysLog(fmt.Sprintf("cloudflare email: rewrote receiver %s -> %s", addr, parts[i]))
		} else {
			parts[i] = addr
		}
	}
	return strings.Join(parts, ";")
}

func SendEmail(subject string, receiver string, content string) error {
	if SMTPFrom == "" { // for compatibility
		SMTPFrom = SMTPAccount
	}
	// Cloudflare Email Service path:
	// 1) Worker relay (recommended from cloud hosts — CF edge sends the mail)
	// 2) Direct REST API (works from many residential networks)
	// 3) Fall through to SMTP (requires domain fully enabled for SMTP submission)
	if isCloudflareEmailService() {
		receiver = normalizeReceiverForCloudflareEmail(receiver)
		if strings.TrimSpace(os.Getenv("CLOUDFLARE_EMAIL_WORKER_URL")) != "" {
			return sendEmailCloudflareWorker(subject, receiver, content)
		}
		if cloudflareAccountID() != "" && cloudflareEmailAPIToken() != "" {
			if err := sendEmailCloudflareREST(subject, receiver, content); err == nil {
				return nil
			} else {
				SysLog(fmt.Sprintf("cloudflare email REST failed: %v", err))
				return err
			}
		}
	}
	id, err2 := generateMessageID()
	if err2 != nil {
		return err2
	}
	if SMTPServer == "" && SMTPAccount == "" {
		return NewMessage("SMTP server is not configured")
	}
	encodedSubject := fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(subject)))
	mail := []byte(fmt.Sprintf("To: %s\r\n"+
		"From: %s <%s>\r\n"+
		"Subject: %s\r\n"+
		"Date: %s\r\n"+
		"Message-ID: %s\r\n"+ // 添加 Message-ID 头
		"Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n",
		receiver, SystemName, SMTPFrom, encodedSubject, time.Now().Format(time.RFC1123Z), id, content))
	auth := getSMTPAuth()
	addr := fmt.Sprintf("%s:%d", SMTPServer, SMTPPort)
	to := strings.Split(receiver, ";")
	var err error
	client, err := newSMTPClient(addr)
	if err != nil {
		return err
	}
	defer client.Close()
	if shouldAuthenticateSMTP() {
		if err = client.Auth(auth); err != nil {
			return err
		}
	}
	if err = client.Mail(SMTPFrom); err != nil {
		return err
	}
	for _, receiver := range to {
		if err = client.Rcpt(receiver); err != nil {
			return err
		}
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(mail)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	err = client.Quit()
	if err != nil {
		SysError(LogText("failed to send email to %s: %v", receiver, err))
	}
	return err
}
