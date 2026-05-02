package crypto

import "testing"

func BenchmarkDeriveKey(b *testing.B) {
	enc := NewEncryptor()
	salt := []byte("0123456789abcdef")

	b.ReportAllocs()
	for b.Loop() {
		_ = enc.DeriveKey("master-password", salt)
	}
}
