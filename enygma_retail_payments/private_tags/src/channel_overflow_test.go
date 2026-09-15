package tags

import (
	"encoding/binary"
	"testing"
)

// TestDecodeChannelPayload_RoundTrip confirms the fix didn't break normal
// encode/decode behavior.
func TestDecodeChannelPayload_RoundTrip(t *testing.T) {
	cases := []ChannelPayload{
		{Message: []byte("hello"), SenderId: []byte("alice")},
		{Message: []byte("hello"), SenderId: nil},
		{Message: nil, SenderId: nil},
		{Message: []byte{}, SenderId: []byte{}},
	}
	for _, want := range cases {
		enc := encodeChannelPayload(want)
		got, err := decodeChannelPayload(enc)
		if err != nil {
			t.Fatalf("decode round-trip failed: %v", err)
		}
		if string(got.Message) != string(want.Message) {
			t.Errorf("Message mismatch: got %q want %q", got.Message, want.Message)
		}
		if len(want.SenderId) > 0 && string(got.SenderId) != string(want.SenderId) {
			t.Errorf("SenderId mismatch: got %q want %q", got.SenderId, want.SenderId)
		}
	}
}

// TestDecodeChannelPayload_OverflowRejected reproduces CWE-190: a msgLen field
// near math.MaxUint32 must be rejected with an error, not panic via an
// out-of-bounds slice (the pre-fix behavior — "4+msgLen+4" wrapped uint32
// arithmetic back around to a small value, letting the bounds check pass
// incorrectly, then data[4:4+msgLen] panicked since 4+msgLen also wrapped
// to a value less than 4).
func TestDecodeChannelPayload_OverflowRejected(t *testing.T) {
	maliciousLens := []uint32{
		0xFFFFFFFF,
		0xFFFFFFFC, // 4 + this wraps to exactly 0
		0xFFFFFFF0,
		0x80000000,
	}
	for _, ml := range maliciousLens {
		data := make([]byte, 20) // plausible small "decrypted" payload
		binary.BigEndian.PutUint32(data[0:4], ml)

		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("decodeChannelPayload panicked on msgLen=0x%x: %v (must return an error instead)", ml, r)
				}
			}()
			_, err := decodeChannelPayload(data)
			if err == nil {
				t.Errorf("decodeChannelPayload accepted malicious msgLen=0x%x, expected an error", ml)
			}
		}()
	}
}

// TestDecodeChannelPayload_SenderIdOverflowRejected covers the second copy of
// the same bug pattern, on the senderIdLen field.
func TestDecodeChannelPayload_SenderIdOverflowRejected(t *testing.T) {
	// A valid, small msgLen=0, followed by a malicious senderIdLen.
	data := make([]byte, 12)
	binary.BigEndian.PutUint32(data[0:4], 0)          // msgLen = 0
	binary.BigEndian.PutUint32(data[4:8], 0xFFFFFFF0) // senderIdLen = huge

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("decodeChannelPayload panicked on malicious senderIdLen: %v (must return an error instead)", r)
		}
	}()
	_, err := decodeChannelPayload(data)
	if err == nil {
		t.Error("decodeChannelPayload accepted malicious senderIdLen, expected an error")
	}
}
