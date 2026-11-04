package composer

import "testing"

func TestByteCountLabelsBytes(t *testing.T) {
	if got := byteCount(1234567); got != "1,234,567 bytes" {
		t.Fatalf("byteCount(1234567): want %q, got %q", "1,234,567 bytes", got)
	}
}
