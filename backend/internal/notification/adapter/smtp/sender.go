package smtp

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net"
	"net/mail"
	netsmtp "net/smtp"
	"strings"
	"time"

	"github.com/thinhnguyenwilliam/book-store/backend/internal/notification/domain"
)

type Config struct {
	Host, Username, Password, FromAddress, FromName string
	Port                                            int
	StartTLS                                        bool
	Timeout                                         time.Duration
}

type Sender struct{ config Config }

func NewSender(config Config) *Sender { return &Sender{config: config} }

func (s *Sender) Send(ctx context.Context, delivery domain.EmailDelivery) error {
	from, err := mail.ParseAddress(s.config.FromAddress)
	if err != nil {
		return fmt.Errorf("invalid SMTP sender: %w", err)
	}
	recipient, err := mail.ParseAddress(delivery.Recipient)
	if err != nil {
		return fmt.Errorf("invalid SMTP recipient: %w", err)
	}
	address := net.JoinHostPort(s.config.Host, fmt.Sprintf("%d", s.config.Port))
	connection, err := (&net.Dialer{Timeout: s.config.Timeout}).DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect SMTP: %w", err)
	}
	defer func() { _ = connection.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	} else {
		_ = connection.SetDeadline(time.Now().Add(s.config.Timeout))
	}
	client, err := netsmtp.NewClient(connection, s.config.Host)
	if err != nil {
		return fmt.Errorf("create SMTP client: %w", err)
	}
	defer func() { _ = client.Close() }()
	if s.config.StartTLS {
		if err := client.StartTLS(&tls.Config{ServerName: s.config.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	}
	if s.config.Username != "" {
		if err := client.Auth(netsmtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)); err != nil {
			return fmt.Errorf("authenticate SMTP: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	if err := client.Rcpt(recipient.Address); err != nil {
		return fmt.Errorf("set SMTP recipient: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("open SMTP data: %w", err)
	}
	message := buildMessage(s.config, delivery)
	if _, err = io.Copy(w, bufio.NewReader(strings.NewReader(message))); err != nil {
		_ = w.Close()
		return fmt.Errorf("write SMTP data: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close SMTP data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("quit SMTP: %w", err)
	}
	return nil
}

func buildMessage(config Config, delivery domain.EmailDelivery) string {
	clean := func(value string) string { return strings.ReplaceAll(strings.ReplaceAll(value, "\r", ""), "\n", "") }
	fromName := mime.QEncoding.Encode("UTF-8", clean(config.FromName))
	subject := mime.QEncoding.Encode("UTF-8", clean(delivery.Subject))
	return fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", fromName, clean(config.FromAddress), clean(delivery.Recipient), subject, strings.ReplaceAll(delivery.Body, "\n", "\r\n"))
}
