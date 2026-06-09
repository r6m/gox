package validx

import "testing"

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
