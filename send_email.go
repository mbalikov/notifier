package main

import (
	"crypto/tls"
	"net"
	"net/smtp"
	"time"
)

/*
 * Timeout version of net/smtp SendMail()
 */
func sendEmail(smtpHost, smtpPort, smtpUser, smtpPass,
	from string, to []string, message []byte, timeout time.Duration) error {

	// Setup a dialer with a timeout.
	dialer := &net.Dialer{
		Timeout:   timeout,
		Deadline:  time.Now().Add(timeout),
		KeepAlive: timeout,
	}

	conn, err := dialer.Dial("tcp", smtpHost+":"+smtpPort)
	if err != nil {
		return err
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, smtpHost)
	if err != nil {
		return err
	}
	defer c.Close()

	// EHLO/HELO
	if err = c.Hello("localhost"); err != nil {
		return err
	}

	// STARTTLS
	if ok, _ := c.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: smtpHost}
		if err = c.StartTLS(config); err != nil {
			return err
		}
	}

	// AUTH
	if smtpUser != "" && smtpPass != "" {
		if ok, _ := c.Extension("AUTH"); ok {
			auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
			if err := c.Auth(auth); err != nil {
				return err
			}
		}
	}

	// MAIL FROM
	if err := c.Mail(from); err != nil {
		return err
	}
	// MAIL TO
	for _, addr := range to {
		if err := c.Rcpt(addr); err != nil {
			return err
		}
	}

	// Send the email body.
	wc, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(message); err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}

	return c.Quit()
}
