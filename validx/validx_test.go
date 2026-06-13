package validx

import (
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestStructMessagesAndJSONNames(t *testing.T) {
	type input struct {
		Email string `json:"email" validate:"required,email"`
		Name  string `json:"name" validate:"min=3,max=5"`
	}
	err := Struct(input{Name: "ab"})
	fields := Fields(err)
	if fields["email"] != "is required" || fields["name"] != "must be at least 3 characters" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestIndependentValidator(t *testing.T) {
	first := New()
	second := New()
	if err := first.RegisterValidation("even", func(fl validator.FieldLevel) bool {
		return fl.Field().Int()%2 == 0
	}); err != nil {
		t.Fatal(err)
	}
	if err := second.RegisterValidation("even", func(validator.FieldLevel) bool {
		return true
	}); err != nil {
		t.Fatal(err)
	}
	type input struct {
		Number int `validate:"even"`
	}
	if err := first.Struct(input{Number: 3}); err == nil {
		t.Fatal("expected custom validation error")
	}
	if err := second.Struct(input{Number: 3}); err != nil {
		t.Fatalf("validator registrations leaked between instances: %v", err)
	}
}

func TestEmailAndMax(t *testing.T) {
	type input struct {
		Email string `json:"email" validate:"email"`
		Name  string `json:"name" validate:"max=3"`
	}
	fields := Fields(Struct(input{Email: "bad", Name: "long"}))
	if fields["email"] != "must be a valid email" || fields["name"] != "must be at most 3 characters" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}

func TestNestedJSONNames(t *testing.T) {
	type profile struct {
		Name string `json:"name" validate:"required"`
	}
	type input struct {
		Profile profile `json:"profile" validate:"required"`
	}
	fields := Fields(Struct(input{}))
	if fields["profile.name"] != "is required" {
		t.Fatalf("unexpected fields: %#v", fields)
	}
}
