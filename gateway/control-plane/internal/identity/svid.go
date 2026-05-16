package identity

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net/url"
	"os"
	"time"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

type SVID struct {
	CertChain  [][]byte
	PrivateKey crypto.Signer
	ExpiresAt  time.Time
	TrustDomain string
	SpiffeID   string
}

type IdentityProvider interface {
	FetchSVID(ctx context.Context) (*SVID, error)
	WriteCerts(certPath, keyPath string) error
}

type LocalProvider struct {
	trustDomain string
	key         *ecdsa.PrivateKey
	cert        *x509.Certificate
	certDER     []byte
}

func NewLocalProvider(trustDomain string) *LocalProvider {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate local key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"AgentID Gateway"},
			CommonName:   "gateway.agentid.dev",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		URIs: []*url.URL{
			{Scheme: "spiffe", Host: trustDomain, Path: "/gateway"},
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		log.Fatalf("Failed to create local certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		log.Fatalf("Failed to parse local certificate: %v", err)
	}

	return &LocalProvider{
		trustDomain: trustDomain,
		key:         key,
		cert:        cert,
		certDER:     certDER,
	}
}

func (l *LocalProvider) FetchSVID(ctx context.Context) (*SVID, error) {
	spiffeID := fmt.Sprintf("spiffe://%s/gateway", l.trustDomain)

	return &SVID{
		CertChain:   [][]byte{l.certDER},
		PrivateKey:  l.key,
		ExpiresAt:   l.cert.NotAfter,
		TrustDomain: l.trustDomain,
		SpiffeID:    spiffeID,
	}, nil
}

func (l *LocalProvider) WriteCerts(certPath, keyPath string) error {
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: l.certDER,
	})
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to write cert: %w", err)
	}

	keyBytes, err := x509.MarshalECPrivateKey(l.key)
	if err != nil {
		return fmt.Errorf("failed to marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write key: %w", err)
	}

	log.Printf("Wrote local development certificates to %s, %s", certPath, keyPath)
	return nil
}

func NewIdentityProvider() IdentityProvider {
	socketPath := os.Getenv("SPIRE_SOCKET_PATH")
	if socketPath == "" {
		socketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	spiffeSocket := os.Getenv("SPIFFE_ENDPOINT_SOCKET")
	if spiffeSocket == "" {
		if socketPath != "" {
			spiffeSocket = socketPath
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(workloadapi.WithAddr(spiffeSocket)),
	)
	if err == nil {
		_ = source.Close()
		log.Printf("SPIRE workload API available at %s, using SPIRE provider", spiffeSocket)
		return NewSpireProvider(spiffeSocket)
	}

	log.Printf("SPIRE workload API not available (%v), using local development identity provider", err)
	return NewLocalProvider("agentid.dev")
}