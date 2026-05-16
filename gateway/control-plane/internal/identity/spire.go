package identity

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

type SpireProvider struct {
	socketPath string
}

func NewSpireProvider(socketPath string) *SpireProvider {
	return &SpireProvider{socketPath: socketPath}
}

func (s *SpireProvider) FetchSVID(ctx context.Context) (*SVID, error) {
	source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(workloadapi.WithAddr(s.socketPath)),
	)
	if err != nil {
		return nil, fmt.Errorf("spire workload api connect: %w", err)
	}
	defer source.Close()

	svid, err := source.GetX509SVID()
	if err != nil {
		return nil, fmt.Errorf("spire fetch svid: %w", err)
	}

	return spiffeSVIDToInternal(svid), nil
}

func (s *SpireProvider) WatchX509SVID(ctx context.Context) (<-chan *SVID, error) {
	ch := make(chan *SVID, 1)

	source, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(workloadapi.WithAddr(s.socketPath)),
	)
	if err != nil {
		return nil, fmt.Errorf("spire workload api watch connect: %w", err)
	}

	go func() {
		defer close(ch)
		defer source.Close()

		updates := source.Updated()
		for {
			select {
			case <-ctx.Done():
				return
			case <-updates:
				svid, err := source.GetX509SVID()
				if err != nil {
				slog.Warn("SPIRE watch fetch error", "error", err)
					continue
				}

				internal := spiffeSVIDToInternal(svid)

				select {
				case ch <- internal:
				case <-ctx.Done():
					return
				}

				if err := source.WaitUntilUpdated(ctx); err != nil {
				slog.Warn("SPIRE watch wait error", "error", err)
					return
				}
			}
		}
	}()

	return ch, nil
}

func (s *SpireProvider) WriteCerts(certPath, keyPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	svid, err := s.FetchSVID(ctx)
	if err != nil {
		return fmt.Errorf("spire fetch svid for cert write: %w", err)
	}

	return writeSVIDCerts(svid, certPath, keyPath)
}

func spiffeSVIDToInternal(svid *x509svid.SVID) *SVID {
	spiffeIDStr := svid.ID.String()
	trustDomain := svid.ID.TrustDomain().String()

	leafCert := svid.Certificates[0]

	var certDER [][]byte
	for _, c := range svid.Certificates {
		certDER = append(certDER, c.Raw)
	}

	return &SVID{
		CertChain:   certDER,
		PrivateKey:  svid.PrivateKey,
		ExpiresAt:   leafCert.NotAfter,
		TrustDomain: trustDomain,
		SpiffeID:    spiffeIDStr,
	}
}

func writeSVIDCerts(svid *SVID, certPath, keyPath string) error {
	var certPEM []byte
	for _, der := range svid.CertChain {
		certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: der,
		})...)
	}
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to write cert: %w", err)
	}

	if signer, ok := svid.PrivateKey.(crypto.Signer); ok {
		keyDER, err := x509.MarshalPKCS8PrivateKey(signer)
		if err != nil {
			return fmt.Errorf("failed to marshal private key: %w", err)
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: keyDER,
		})
		if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
			return fmt.Errorf("failed to write key: %w", err)
		}
	}

	slog.Info("wrote SVID certificates", "cert_path", certPath, "key_path", keyPath)
	return nil
}