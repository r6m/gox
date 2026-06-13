package emailx

import (
	"context"
	"sync"
	"time"
)

// Recorder is a race-safe in-memory Sender for deterministic tests.
type Recorder struct {
	mu       sync.Mutex
	messages []Message
	err      error
}

// NewRecorder creates an empty in-memory sender.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Send records a defensive copy of message.
func (r *Recorder) Send(ctx context.Context, message Message) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := validateMessage(message); err != nil {
		return Result{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return Result{}, r.err
	}
	r.messages = append(r.messages, cloneMessage(message))
	return Result{SentAt: time.Now().UTC()}, nil
}

// Messages returns defensive copies of recorded messages.
func (r *Recorder) Messages() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	messages := make([]Message, len(r.messages))
	for index, message := range r.messages {
		messages[index] = cloneMessage(message)
	}
	return messages
}

// SetError configures the error returned by future sends.
func (r *Recorder) SetError(err error) {
	r.mu.Lock()
	r.err = err
	r.mu.Unlock()
}

// Reset clears recorded messages and configured errors.
func (r *Recorder) Reset() {
	r.mu.Lock()
	r.messages = nil
	r.err = nil
	r.mu.Unlock()
}

func cloneMessage(message Message) Message {
	cloned := message
	cloned.To = append([]Address(nil), message.To...)
	cloned.CC = append([]Address(nil), message.CC...)
	cloned.BCC = append([]Address(nil), message.BCC...)
	cloned.ReplyTo = append([]Address(nil), message.ReplyTo...)
	cloned.Attachments = append([]Attachment(nil), message.Attachments...)
	if message.Headers != nil {
		cloned.Headers = make(map[string]string, len(message.Headers))
		for key, value := range message.Headers {
			cloned.Headers[key] = value
		}
	}
	return cloned
}

var _ Sender = (*Recorder)(nil)
