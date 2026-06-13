package pgxutil

import (
	"testing"
	"time"
)

func TestNullableConverters(t *testing.T) {
	text := ""
	if got := Text(&text); !got.Valid || got.String != "" {
		t.Fatalf("unexpected text: %#v", got)
	}
	if Text(nil).Valid || TextPtr(Text(nil)) != nil || *TextPtr(Text(&text)) != "" {
		t.Fatal("text null conversion failed")
	}

	number := int32(0)
	if got := Int4(&number); !got.Valid || got.Int32 != 0 {
		t.Fatalf("unexpected int4: %#v", got)
	}
	if Int4Ptr(Int4(nil)) != nil || *Int4Ptr(Int4(&number)) != 0 {
		t.Fatal("int4 null conversion failed")
	}

	enabled := false
	if got := Bool(&enabled); !got.Valid || got.Bool {
		t.Fatalf("unexpected bool: %#v", got)
	}
	if BoolPtr(Bool(nil)) != nil || *BoolPtr(Bool(&enabled)) {
		t.Fatal("bool null conversion failed")
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	if got := Timestamptz(&now); !got.Valid || !got.Time.Equal(now) {
		t.Fatalf("unexpected timestamptz: %#v", got)
	}
	if TimestamptzPtr(Timestamptz(nil)) != nil || !TimestamptzPtr(Timestamptz(&now)).Equal(now) {
		t.Fatal("timestamptz null conversion failed")
	}

	uuid := [16]byte{1, 2, 3}
	if got := UUID(&uuid); !got.Valid || got.Bytes != uuid {
		t.Fatalf("unexpected uuid: %#v", got)
	}
	if UUIDPtr(UUID(nil)) != nil || *UUIDPtr(UUID(&uuid)) != uuid {
		t.Fatal("uuid null conversion failed")
	}
}
