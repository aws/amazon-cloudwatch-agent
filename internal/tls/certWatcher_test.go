// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tls

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func createRootCert(caPath string) (*x509.Certificate, *rsa.PrivateKey, error) {
	rootKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}

	rootTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Root CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // Valid for 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	rootCertDER, err := x509.CreateCertificate(rand.Reader, &rootTemplate, &rootTemplate, &rootKey.PublicKey, rootKey)
	if err != nil {
		return nil, nil, err
	}

	rootCert, err := x509.ParseCertificate(rootCertDER)
	if err != nil {
		return nil, nil, err
	}

	// Write the root certificate
	rootCertOut, err := os.Create(caPath)
	if err != nil {
		return nil, nil, err
	}
	defer rootCertOut.Close()
	if err := pem.Encode(rootCertOut, &pem.Block{Type: "CERTIFICATE", Bytes: rootCertDER}); err != nil {
		return nil, nil, err
	}

	return rootCert, rootKey, nil
}

func writeCerts(certPath, keyPath, caPath, ip string) error {
	// Generate root certificate
	rootCert, rootKey, err := createRootCert(caPath)
	if err != nil {
		return err
	}

	// Generate key for the actual certificate
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(1 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Kubernetes"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	template.IPAddresses = append(template.IPAddresses, net.ParseIP(ip))

	// Create the certificate using the root certificate as the CA
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, rootCert, &priv.PublicKey, rootKey)
	if err != nil {
		return err
	}

	// Write the certificate
	certOut, err := os.Create(certPath)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return err
	}

	// Write the private key
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return err
	}

	return nil
}

func setupCerts(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")
	caPath := filepath.Join(dir, "tls-ca.crt")
	require.NoError(t, writeCerts(certPath, keyPath, caPath, "127.0.0.1"))
	return certPath, keyPath, caPath
}

func TestNewCertWatcher(t *testing.T) {
	t.Parallel()
	certPath, keyPath, caPath := setupCerts(t)
	logger := zaptest.NewLogger(t)

	cw, err := NewCertWatcher(certPath, keyPath, caPath, logger)
	assert.NoError(t, err, "Failed to create CertWatcher")

	assert.NotNil(t, cw.GetTLSConfig(), "TLS config was not loaded correctly")
}

func TestRegisterCallback(t *testing.T) {
	t.Parallel()
	certPath, keyPath, caPath := setupCerts(t)
	logger := zaptest.NewLogger(t)

	cw, err := NewCertWatcher(certPath, keyPath, caPath, logger)
	assert.NoError(t, err, "Failed to create CertWatcher")

	var callbackCalled atomic.Bool
	callback := func() {
		callbackCalled.Store(true)
	}
	cw.RegisterCallback(callback)

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := cw.Start(ctx)
		assert.NoError(t, err, "Failed to start CertWatcher")
	}()
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	_, _, err = createRootCert(caPath)
	assert.NoError(t, err, "Failed to update root certificate")

	require.Eventually(t, func() bool {
		return callbackCalled.Load()
	}, 1*time.Second, 50*time.Millisecond)
}

func TestWatchCertificateChanges(t *testing.T) {
	t.Parallel()
	certPath, keyPath, caPath := setupCerts(t)
	logger := zaptest.NewLogger(t)

	cw, err := NewCertWatcher(certPath, keyPath, caPath, logger)
	assert.NoError(t, err, "Failed to create CertWatcher")

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := cw.Start(ctx)
		assert.NoError(t, err, "Failed to start CertWatcher")
	}()
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	beforeTLSConfig := cw.GetTLSConfig()

	_, _, err = createRootCert(caPath)
	assert.NoError(t, err, "Failed to update root certificate")

	require.Eventually(t, func() bool {
		return beforeTLSConfig != cw.GetTLSConfig()
	}, 2*time.Second, 50*time.Millisecond, "TLS config was not updated after certificate change")
}
