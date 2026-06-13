package configx

import (
	"encoding"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/caarlos0/env/v11"
)

type customValue string

func (v *customValue) UnmarshalText(text []byte) error {
	*v = customValue(strings.ToUpper(string(text)))
	return nil
}

var _ encoding.TextUnmarshaler = (*customValue)(nil)

type config struct {
	Host    string        `env:"HOST,required"`
	Port    int           `env:"PORT" envDefault:"8080"`
	Timeout time.Duration `env:"TIMEOUT" envDefault:"5s"`
	Custom  customValue   `env:"CUSTOM"`
}

func (c config) Validate() error {
	if c.Port < 1 {
		return errors.New("port must be positive")
	}
	return nil
}

func TestLoad(t *testing.T) {
	got, err := Load[config](Options{
		Prefix: "APP_",
		Environment: map[string]string{
			"APP_HOST":    "localhost",
			"APP_TIMEOUT": "2s",
			"APP_CUSTOM":  "mixed",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "localhost" || got.Port != 8080 || got.Timeout != 2*time.Second || got.Custom != "MIXED" {
		t.Fatalf("unexpected config: %#v", got)
	}
}

func TestLoadRequiredAndValidation(t *testing.T) {
	if _, err := Load[config](Options{Environment: map[string]string{}}); err == nil {
		t.Fatal("expected required value error")
	}
	_, err := Load[config](Options{Environment: map[string]string{"HOST": "localhost", "PORT": "0"}})
	if err == nil || !strings.Contains(err.Error(), "port must be positive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadFuncMap(t *testing.T) {
	type parsed struct {
		Number int64 `env:"NUMBER"`
	}
	got, err := Load[parsed](Options{
		Environment: map[string]string{"NUMBER": "21"},
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeOf(int64(0)): func(value string) (any, error) {
				number, err := strconv.ParseInt(value, 10, 64)
				return number * 2, err
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Number != 42 {
		t.Fatalf("unexpected number: %d", got.Number)
	}
}
