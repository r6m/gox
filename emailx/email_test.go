package emailx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/fstest"
)

func testMessage() Message {
	return Message{
		From:    Address{Name: "Sender", Email: "sender@example.com"},
		To:      []Address{{Name: "Recipient", Email: "recipient@example.com"}},
		Subject: "Subject",
		Text:    "Plain body",
		HTML:    "<strong>HTML body</strong>",
	}
}

func TestRecorder(t *testing.T) {
	recorder := NewRecorder()
	message := testMessage()
	if _, err := recorder.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
	message.To[0].Email = "changed@example.com"
	recorded := recorder.Messages()
	if len(recorded) != 1 || recorded[0].To[0].Email != "recipient@example.com" {
		t.Fatalf("unexpected recorded message: %#v", recorded)
	}
}

func TestMessageValidation(t *testing.T) {
	tests := []Message{
		{},
		{From: Address{Email: "bad"}, To: []Address{{Email: "to@example.com"}}, Text: "body"},
		{From: Address{Email: "from@example.com"}, Text: "body"},
		{
			From:    Address{Email: "from@example.com"},
			To:      []Address{{Email: "to@example.com"}},
			Text:    "body",
			Headers: map[string]string{"Content-Type": "bad"},
		},
	}
	for _, message := range tests {
		if err := validateMessage(message); err == nil {
			t.Fatalf("invalid message accepted: %#v", message)
		}
	}
}

func TestMIMEStreamsAndClosesAttachment(t *testing.T) {
	closed := false
	message := testMessage()
	message.BCC = []Address{{Email: "hidden@example.com"}}
	message.Attachments = []Attachment{{
		Filename:    "report.txt",
		ContentType: "text/plain",
		Open: func() (io.ReadCloser, error) {
			return &trackingReader{
				Reader: strings.NewReader("attachment"),
				closed: &closed,
			}, nil
		},
	}}
	var output bytes.Buffer
	if _, err := writeMIME(context.Background(), &output, message); err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Fatal("attachment reader was not closed")
	}
	content := output.String()
	if strings.Contains(content, "Bcc:") ||
		!strings.Contains(content, `filename=report.txt`) ||
		!strings.Contains(content, "multipart/alternative") {
		t.Fatalf("unexpected MIME message:\n%s", content)
	}
}

func TestTemplateRenderer(t *testing.T) {
	fsys := fstest.MapFS{
		"welcome.subject": {Data: []byte(`Welcome {{.Name}}`)},
		"welcome.txt":     {Data: []byte(`Hello {{.Name}}`)},
		"welcome.html":    {Data: []byte(`<p>Hello {{.Name}}</p>`)},
	}
	renderer, err := NewTemplateRenderer(fsys, []TemplateSpec{{
		Name:        "welcome",
		Message:     testMessage(),
		SubjectPath: "welcome.subject",
		TextPath:    "welcome.txt",
		HTMLPath:    "welcome.html",
	}})
	if err != nil {
		t.Fatal(err)
	}
	message, err := renderer.Render(context.Background(), "welcome", struct{ Name string }{Name: "<User>"})
	if err != nil {
		t.Fatal(err)
	}
	if message.Subject != "Welcome <User>" ||
		message.Text != "Hello <User>" ||
		message.HTML != "<p>Hello &lt;User&gt;</p>" {
		t.Fatalf("unexpected rendered message: %#v", message)
	}
	if _, err := renderer.Render(context.Background(), "missing", nil); err == nil {
		t.Fatal("missing template accepted")
	}
	if _, err := renderer.Render(context.Background(), "welcome", struct{}{}); err == nil {
		t.Fatal("missing template data accepted")
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewRecorder().Send(ctx, testMessage()); !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected cancellation: %v", err)
	}
}

type trackingReader struct {
	io.Reader
	closed *bool
}

func (r *trackingReader) Close() error {
	*r.closed = true
	return nil
}
