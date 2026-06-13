package emailx

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// SMTPOptions configures SMTP delivery.
type SMTPOptions struct {
	Host        string
	Port        int
	Username    string
	Password    string
	ImplicitTLS bool
	// AllowInsecure permits cleartext SMTP when STARTTLS is unavailable.
	// It should be used only for trusted development servers.
	AllowInsecure      bool
	InsecureSkipVerify bool
	DialTimeout        time.Duration
}

// SMTPSender delivers messages through SMTP.
type SMTPSender struct {
	options SMTPOptions
	address string
}

// NewSMTPSender creates a validated SMTP sender.
func NewSMTPSender(opts SMTPOptions) (*SMTPSender, error) {
	if opts.Host == "" {
		return nil, errors.New("emailx: SMTP host is required")
	}
	if opts.Port < 1 || opts.Port > 65535 {
		return nil, errors.New("emailx: SMTP port must be between 1 and 65535")
	}
	if (opts.Username == "") != (opts.Password == "") {
		return nil, errors.New("emailx: SMTP username and password must be configured together")
	}
	if opts.DialTimeout <= 0 {
		opts.DialTimeout = 10 * time.Second
	}
	return &SMTPSender{
		options: opts,
		address: net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port)),
	}, nil
}

// Send renders MIME content and delivers it through SMTP.
func (s *SMTPSender) Send(ctx context.Context, message Message) (Result, error) {
	if err := validateMessage(message); err != nil {
		return Result{}, err
	}
	connection, err := s.dial(ctx)
	if err != nil {
		return Result{}, err
	}
	defer func() { _ = connection.Close() }()
	stopCancellation := closeOnCancellation(ctx, connection)
	defer stopCancellation()

	client, err := smtp.NewClient(connection, s.options.Host)
	if err != nil {
		return Result{}, fmt.Errorf("emailx: create SMTP client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if !s.options.ImplicitTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(s.tlsConfig()); err != nil {
				return Result{}, fmt.Errorf("emailx: start SMTP TLS: %w", err)
			}
		} else if !s.options.AllowInsecure {
			return Result{}, errors.New("emailx: SMTP server does not support STARTTLS")
		}
	}
	if s.options.Username != "" {
		if err := client.Auth(smtp.PlainAuth(
			"",
			s.options.Username,
			s.options.Password,
			s.options.Host,
		)); err != nil {
			return Result{}, fmt.Errorf("emailx: authenticate SMTP: %w", err)
		}
	}
	if err := client.Mail(message.From.Email); err != nil {
		return Result{}, fmt.Errorf("emailx: set SMTP sender: %w", err)
	}
	for _, recipient := range recipients(message) {
		if err := client.Rcpt(recipient.Email); err != nil {
			return Result{}, fmt.Errorf("emailx: set SMTP recipient %q: %w", recipient.Email, err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return Result{}, fmt.Errorf("emailx: open SMTP data: %w", err)
	}
	messageID, err := writeMIME(ctx, writer, message)
	closeErr := writer.Close()
	if err != nil {
		return Result{}, err
	}
	if closeErr != nil {
		return Result{}, fmt.Errorf("emailx: close SMTP data: %w", closeErr)
	}
	if err := client.Quit(); err != nil {
		return Result{}, fmt.Errorf("emailx: quit SMTP session: %w", err)
	}
	return Result{MessageID: messageID, SentAt: time.Now().UTC()}, nil
}

func (s *SMTPSender) dial(ctx context.Context) (net.Conn, error) {
	dialer := net.Dialer{Timeout: s.options.DialTimeout}
	if s.options.ImplicitTLS {
		connection, err := tls.DialWithDialer(&dialer, "tcp", s.address, s.tlsConfig())
		if err != nil {
			return nil, fmt.Errorf("emailx: dial SMTP TLS: %w", err)
		}
		return connection, nil
	}
	connection, err := dialer.DialContext(ctx, "tcp", s.address)
	if err != nil {
		return nil, fmt.Errorf("emailx: dial SMTP: %w", err)
	}
	return connection, nil
}

func closeOnCancellation(ctx context.Context, connection net.Conn) func() {
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	stopped := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = connection.Close()
		case <-stopped:
		}
	}()
	return func() {
		close(stopped)
		_ = connection.SetDeadline(time.Time{})
	}
}

func (s *SMTPSender) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName:         s.options.Host,
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: s.options.InsecureSkipVerify, //nolint:gosec // Explicit test/server option.
	}
}

func writeMIME(ctx context.Context, dst io.Writer, message Message) (string, error) {
	messageID, err := newMessageID(message.From.Email)
	if err != nil {
		return "", err
	}
	buffered := bufio.NewWriter(dst)
	headers := textproto.MIMEHeader{}
	headers.Set("From", message.From.String())
	headers.Set("To", joinAddresses(message.To))
	if len(message.CC) > 0 {
		headers.Set("Cc", joinAddresses(message.CC))
	}
	if len(message.ReplyTo) > 0 {
		headers.Set("Reply-To", joinAddresses(message.ReplyTo))
	}
	headers.Set("Subject", mime.QEncoding.Encode("utf-8", message.Subject))
	headers.Set("Date", time.Now().Format(time.RFC1123Z))
	headers.Set("Message-ID", messageID)
	headers.Set("MIME-Version", "1.0")
	for name, value := range message.Headers {
		headers.Set(name, value)
	}

	mixedBoundary := randomBoundary()
	alternativeBoundary := randomBoundary()
	if len(message.Attachments) > 0 {
		headers.Set("Content-Type", `multipart/mixed; boundary="`+mixedBoundary+`"`)
	} else if message.Text != "" && message.HTML != "" {
		headers.Set("Content-Type", `multipart/alternative; boundary="`+alternativeBoundary+`"`)
	} else {
		contentType := "text/plain"
		if message.HTML != "" {
			contentType = "text/html"
		}
		headers.Set("Content-Type", contentType+`; charset="utf-8"`)
		headers.Set("Content-Transfer-Encoding", "quoted-printable")
	}
	if err := writeHeaders(buffered, headers); err != nil {
		return "", err
	}

	if len(message.Attachments) > 0 {
		mixed := multipart.NewWriter(buffered)
		if err := mixed.SetBoundary(mixedBoundary); err != nil {
			return "", err
		}
		if err := writeBodyParts(ctx, mixed, alternativeBoundary, message); err != nil {
			return "", err
		}
		for _, attachment := range message.Attachments {
			if err := writeAttachment(ctx, mixed, attachment); err != nil {
				return "", err
			}
		}
		if err := mixed.Close(); err != nil {
			return "", fmt.Errorf("emailx: close MIME body: %w", err)
		}
	} else if message.Text != "" && message.HTML != "" {
		alternative := multipart.NewWriter(buffered)
		if err := alternative.SetBoundary(alternativeBoundary); err != nil {
			return "", err
		}
		if err := writeAlternative(ctx, alternative, message); err != nil {
			return "", err
		}
		if err := alternative.Close(); err != nil {
			return "", fmt.Errorf("emailx: close alternative body: %w", err)
		}
	} else {
		body := message.Text
		if message.HTML != "" {
			body = message.HTML
		}
		if err := writeQuotedPrintable(ctx, buffered, strings.NewReader(body)); err != nil {
			return "", err
		}
	}
	if err := buffered.Flush(); err != nil {
		return "", fmt.Errorf("emailx: flush MIME message: %w", err)
	}
	return messageID, nil
}

func writeBodyParts(
	ctx context.Context,
	mixed *multipart.Writer,
	alternativeBoundary string,
	message Message,
) error {
	if message.Text != "" && message.HTML != "" {
		header := textproto.MIMEHeader{}
		header.Set("Content-Type", `multipart/alternative; boundary="`+alternativeBoundary+`"`)
		part, err := mixed.CreatePart(header)
		if err != nil {
			return err
		}
		alternative := multipart.NewWriter(part)
		if err := alternative.SetBoundary(alternativeBoundary); err != nil {
			return err
		}
		if err := writeAlternative(ctx, alternative, message); err != nil {
			return err
		}
		return alternative.Close()
	}
	contentType, body := "text/plain", message.Text
	if message.HTML != "" {
		contentType, body = "text/html", message.HTML
	}
	return writeTextPart(ctx, mixed, contentType, body)
}

func writeAlternative(ctx context.Context, writer *multipart.Writer, message Message) error {
	if err := writeTextPart(ctx, writer, "text/plain", message.Text); err != nil {
		return err
	}
	return writeTextPart(ctx, writer, "text/html", message.HTML)
}

func writeTextPart(ctx context.Context, writer *multipart.Writer, contentType, body string) error {
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", contentType+`; charset="utf-8"`)
	header.Set("Content-Transfer-Encoding", "quoted-printable")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("emailx: create MIME text part: %w", err)
	}
	return writeQuotedPrintable(ctx, part, strings.NewReader(body))
}

func writeAttachment(ctx context.Context, writer *multipart.Writer, attachment Attachment) error {
	body, err := attachment.Open()
	if err != nil {
		return fmt.Errorf("emailx: open attachment %q: %w", attachment.Filename, err)
	}
	defer func() { _ = body.Close() }()
	contentType := attachment.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Type", contentType)
	header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": attachment.Filename,
	}))
	header.Set("Content-Transfer-Encoding", "base64")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("emailx: create attachment %q: %w", attachment.Filename, err)
	}
	encoder := base64.NewEncoder(base64.StdEncoding, &lineWriter{writer: part})
	_, copyErr := copyWithContext(ctx, encoder, body)
	closeErr := encoder.Close()
	if copyErr != nil {
		return fmt.Errorf("emailx: write attachment %q: %w", attachment.Filename, copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("emailx: encode attachment %q: %w", attachment.Filename, closeErr)
	}
	return nil
}

func writeQuotedPrintable(ctx context.Context, dst io.Writer, src io.Reader) error {
	writer := quotedprintable.NewWriter(dst)
	_, copyErr := copyWithContext(ctx, writer, src)
	closeErr := writer.Close()
	if copyErr != nil {
		return fmt.Errorf("emailx: write MIME body: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("emailx: encode MIME body: %w", closeErr)
	}
	return nil
}

func writeHeaders(writer *bufio.Writer, headers textproto.MIMEHeader) error {
	for name, values := range headers {
		for _, value := range values {
			if _, err := fmt.Fprintf(writer, "%s: %s\r\n", name, value); err != nil {
				return fmt.Errorf("emailx: write MIME header: %w", err)
			}
		}
	}
	_, err := writer.WriteString("\r\n")
	return err
}

func recipients(message Message) []Address {
	result := make([]Address, 0, len(message.To)+len(message.CC)+len(message.BCC))
	result = append(result, message.To...)
	result = append(result, message.CC...)
	result = append(result, message.BCC...)
	return result
}

func joinAddresses(addresses []Address) string {
	values := make([]string, len(addresses))
	for index, address := range addresses {
		values[index] = address.String()
	}
	return strings.Join(values, ", ")
}

func newMessageID(from string) (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", fmt.Errorf("emailx: generate message ID: %w", err)
	}
	domain := "localhost"
	if address, err := mail.ParseAddress(from); err == nil {
		if index := strings.LastIndexByte(address.Address, '@'); index >= 0 {
			domain = address.Address[index+1:]
		}
	}
	return fmt.Sprintf("<%x@%s>", data, domain), nil
}

func randomBoundary() string {
	var data [16]byte
	_, _ = rand.Read(data[:])
	return fmt.Sprintf("%x", data)
}

func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buffer := make([]byte, 32*1024)
	var written int64
	for {
		if err := ctx.Err(); err != nil {
			return written, err
		}
		read, readErr := src.Read(buffer)
		if read > 0 {
			count, writeErr := dst.Write(buffer[:read])
			written += int64(count)
			if writeErr != nil {
				return written, writeErr
			}
			if count != read {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return written, nil
			}
			return written, readErr
		}
	}
}

type lineWriter struct {
	writer io.Writer
	column int
}

func (w *lineWriter) Write(data []byte) (int, error) {
	total := 0
	for len(data) > 0 {
		remaining := 76 - w.column
		if remaining == 0 {
			if _, err := io.WriteString(w.writer, "\r\n"); err != nil {
				return total, err
			}
			w.column = 0
			remaining = 76
		}
		count := min(len(data), remaining)
		written, err := w.writer.Write(data[:count])
		total += written
		w.column += written
		data = data[written:]
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

var _ Sender = (*SMTPSender)(nil)
