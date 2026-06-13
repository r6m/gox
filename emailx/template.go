package emailx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	texttemplate "text/template"
)

// Renderer converts named template input into a normal Message.
type Renderer interface {
	Render(ctx context.Context, name string, data any) (Message, error)
}

// TemplateSpec describes one pre-parsed email template.
type TemplateSpec struct {
	Name        string
	Message     Message
	SubjectPath string
	TextPath    string
	HTMLPath    string
}

// TemplateRenderer renders pre-parsed standard-library templates.
type TemplateRenderer struct {
	templates map[string]compiledTemplate
}

type compiledTemplate struct {
	message Message
	subject *texttemplate.Template
	text    *texttemplate.Template
	html    *template.Template
}

// NewTemplateRenderer parses templates from fsys.
func NewTemplateRenderer(fsys fs.FS, specs []TemplateSpec) (*TemplateRenderer, error) {
	if fsys == nil {
		return nil, errors.New("emailx: template filesystem is required")
	}
	renderer := &TemplateRenderer{templates: make(map[string]compiledTemplate, len(specs))}
	for _, spec := range specs {
		if spec.Name == "" {
			return nil, errors.New("emailx: template name is required")
		}
		if _, exists := renderer.templates[spec.Name]; exists {
			return nil, fmt.Errorf("emailx: duplicate template %q", spec.Name)
		}
		compiled := compiledTemplate{message: cloneMessage(spec.Message)}
		var err error
		if spec.SubjectPath != "" {
			compiled.subject, err = parseTextTemplate(fsys, spec.SubjectPath)
			if err != nil {
				return nil, fmt.Errorf("emailx: parse subject template %q: %w", spec.Name, err)
			}
		}
		if spec.TextPath != "" {
			compiled.text, err = parseTextTemplate(fsys, spec.TextPath)
			if err != nil {
				return nil, fmt.Errorf("emailx: parse text template %q: %w", spec.Name, err)
			}
		}
		if spec.HTMLPath != "" {
			compiled.html, err = parseHTMLTemplate(fsys, spec.HTMLPath)
			if err != nil {
				return nil, fmt.Errorf("emailx: parse HTML template %q: %w", spec.Name, err)
			}
		}
		if compiled.subject == nil && compiled.text == nil && compiled.html == nil {
			return nil, fmt.Errorf("emailx: template %q has no template files", spec.Name)
		}
		renderer.templates[spec.Name] = compiled
	}
	return renderer, nil
}

// Render executes a named template. Parsed templates are safe for concurrent
// use; each render writes to independent buffers.
func (r *TemplateRenderer) Render(ctx context.Context, name string, data any) (Message, error) {
	if err := ctx.Err(); err != nil {
		return Message{}, err
	}
	compiled, ok := r.templates[name]
	if !ok {
		return Message{}, fmt.Errorf("emailx: template %q not found", name)
	}
	message := cloneMessage(compiled.message)
	var err error
	if compiled.subject != nil {
		message.Subject, err = executeText(compiled.subject, data)
		if err != nil {
			return Message{}, fmt.Errorf("emailx: execute subject template %q: %w", name, err)
		}
	}
	if compiled.text != nil {
		message.Text, err = executeText(compiled.text, data)
		if err != nil {
			return Message{}, fmt.Errorf("emailx: execute text template %q: %w", name, err)
		}
	}
	if compiled.html != nil {
		var buffer bytes.Buffer
		if err := compiled.html.Execute(&buffer, data); err != nil {
			return Message{}, fmt.Errorf("emailx: execute HTML template %q: %w", name, err)
		}
		message.HTML = buffer.String()
	}
	return message, nil
}

func parseTextTemplate(fsys fs.FS, filename string) (*texttemplate.Template, error) {
	data, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return nil, err
	}
	return texttemplate.New(path.Base(filename)).Option("missingkey=error").Parse(string(data))
}

func parseHTMLTemplate(fsys fs.FS, filename string) (*template.Template, error) {
	data, err := fs.ReadFile(fsys, filename)
	if err != nil {
		return nil, err
	}
	return template.New(path.Base(filename)).Option("missingkey=error").Parse(string(data))
}

func executeText(template *texttemplate.Template, data any) (string, error) {
	var buffer bytes.Buffer
	if err := template.Execute(&buffer, data); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

var _ Renderer = (*TemplateRenderer)(nil)
