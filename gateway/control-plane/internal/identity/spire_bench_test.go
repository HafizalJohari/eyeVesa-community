package identity

import (
	"encoding/json"
	"testing"
)

func BenchmarkValidateBundleJWKSet(b *testing.B) {
	jwkSet := `{"keys":[{"kty":"EC","crv":"P-256","x":"test","y":"test"}]}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := validateBundle(jwkSet); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateBundleInvalid(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validateBundle("not valid at all")
	}
}

func BenchmarkValidateBundleEmpty(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validateBundle("")
	}
}

func BenchmarkParseSelectorsJSON(b *testing.B) {
	jsonStr := `["unix:uid:1000","k8s:ns:default","docker:image:nginx"]`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseSelectors(jsonStr)
	}
}

func BenchmarkParseSelectorsStringSlice(b *testing.B) {
	slice := []string{"unix:uid:1000", "k8s:ns:default", "docker:image:nginx"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parseSelectors(slice)
	}
}

func BenchmarkTrustBundleJSONMarshal(b *testing.B) {
	bundle := TrustBundle{
		BundleID:       "bench-id",
		TrustDomain:    "bench.example.com",
		BundleData:     `{"keys":[{"kty":"EC","crv":"P-256","x":"bench","y":"bench"}]}`,
		BundleType:     "spiffe_x509",
		Source:         "static",
		SequenceNumber: 42,
		IsFederated:    false,
		Verified:       true,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(bundle)
	}
}