// Package emailx provides email composition, rendering, and delivery.
package emailx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strings"
	"time"
)

// Address identifies an email recipient or sender.
type Address struct {
	Name  string
	Email string
}

// String formats the address for an RFC 5322 header.
func (a Address) String() string {
	return (&mail.Address{Name: a.Name, Address: a.Email}).String()
}

// Attachment describes a repeatable attachment source. Senders call Open once
// per delivery and close the returned reader.
type Attachment struct {
	Filename    string
	ContentType string
	Open        func() (io.ReadCloser, error)
}

// Message is a fully rendered email.
type Message struct {
	From        Address
	To          []Address
	CC          []Address
	BCC         []Address
	ReplyTo     []Address
	Subject     string
	Text        string
	HTML        string
	Headers     map[string]string
	Attachments []Attachment
}

// Result contains provider-neutral delivery information.
type Result struct {
	MessageID string
	SentAt    time.Time
}

// Sender delivers rendered messages.
type Sender interface {
	Send(ctx context.Context, message Message) (Result, error)
}

var reservedHeaders = map[string]bool{
	"bcc":                       true,
	"cc":                        true,
	"content-transfer-encoding": true,
	"content-type":              true,
	"date":                      true,
	"from":                      true,
	"message-id":                true,
	"mime-version":              true,
	"reply-to":                  true,
	"subject":                   true,
	"to":                        true,
}

func validateMessage(message Message) error {
	if err := validateAddress("from", message.From); err != nil {
		return err
	}
	if len(message.To)+len(message.CC)+len(message.BCC) == 0 {
		return errors.New("emailx: at least one recipient is required")
	}
	for _, group := range []struct {
		name      string
		addresses []Address
	}{
		{"to", message.To},
		{"cc", message.CC},
		{"bcc", message.BCC},
		{"reply-to", message.ReplyTo},
	} {
		for _, address := range group.addresses {
			if err := validateAddress(group.name, address); err != nil {
				return err
			}
		}
	}
	if message.Text == "" && message.HTML == "" {
		return errors.New("emailx: text or HTML body is required")
	}
	for name, value := range message.Headers {
		if reservedHeaders[strings.ToLower(name)] {
			return fmt.Errorf("emailx: header %q is reserved", name)
		}
		if !validHeader(name) || strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("emailx: invalid header %q", name)
		}
	}
	for _, attachment := range message.Attachments {
		if attachment.Filename == "" || strings.ContainsAny(attachment.Filename, "\r\n") {
			return errors.New("emailx: attachment filename is invalid")
		}
		if attachment.Open == nil {
			return fmt.Errorf("emailx: attachment %q has no reader factory", attachment.Filename)
		}
	}
	return nil
}

func validateAddress(field string, address Address) error {
	if address.Email == "" {
		return fmt.Errorf("emailx: %s address is required", field)
	}
	parsed, err := mail.ParseAddress(address.String())
	if err != nil || parsed.Address != address.Email {
		return fmt.Errorf("emailx: invalid %s address %q", field, address.Email)
	}
	return nil
}

func validHeader(name string) bool {
	if name == "" {
		return false
	}
	for _, character := range name {
		if character <= 32 || character >= 127 || character == ':' {
			return false
		}
	}
	return true
}
