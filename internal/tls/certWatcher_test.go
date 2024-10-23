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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

const (
	// Set up paths for test certificates
	testCertPath = "./testdata/server.crt"
	testKeyPath  = "./testdata/server.key"
	testCAPath   = "./testdata/tls-ca.crt"
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

func TestMain(m *testing.M) {
	// Setup
	err := Init()
	if err != nil {
		panic(err)
	}

	// Run tests
	code := m.Run()

	// Teardown (if needed)
	// You can add cleanup code here

	// Exit with the test result code
	os.Exit(code)
}

func Init() error {

	// Generate test certificates
	err := writeCerts(testCertPath, testKeyPath, testCAPath, "127.0.0.1")
	if err != nil {
		return err
	}

	return nil
}

func TestNewCertWatcher(t *testing.T) {
	// Setup
	logger := zaptest.NewLogger(t)

	// Test case: Create a new CertWatcher
	cw, err := NewCertWatcher(testCertPath, testKeyPath, testCAPath, logger)
	assert.NoError(t, err, "Failed to create CertWatcher")

	// Check if the initial TLS config was loaded correctly
	assert.NotNil(t, cw.GetTLSConfig(), "TLS config was not loaded correctly")
}

func TestRegisterCallback(t *testing.T) {
	// Setup
	logger := zaptest.NewLogger(t)

	cw, err := NewCertWatcher(testCertPath, testKeyPath, testCAPath, logger)
	assert.NoError(t, err, "Failed to create CertWatcher")

	// Test case: Register a callback
	callbackCalled := false
	callback := func() {
		callbackCalled = true
	}
	cw.RegisterCallback(callback)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := cw.Start(ctx)
		assert.NoError(t, err, "Failed to start CertWatcher")
	}()

	// Trigger a certificate change event
	_, _, err = createRootCert(testCAPath)
	assert.NoError(t, err, "Failed to update root certificate")

	// Wait for the callback to be called
	time.Sleep(1 * time.Second)

	assert.True(t, callbackCalled)
}

func TestWatchCertificateChanges(t *testing.T) {
	// Setup
	logger := zaptest.NewLogger(t)

	cw, err := NewCertWatcher(testCertPath, testKeyPath, testCAPath, logger)
	assert.NoError(t, err, "Failed to create CertWatcher")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		err := cw.Start(ctx)
		assert.NoError(t, err, "Failed to start CertWatcher")
	}()

	beforeTLSConfig := cw.GetTLSConfig()

	// Trigger a certificate change event
	_, _, err = createRootCert(testCAPath)
	assert.NoError(t, err, "Failed to update root certificate")

	// Wait for the certificate change to be processed
	time.Sleep(2 * time.Second)

	// Check if the TLS config was updated
	assert.True(t, beforeTLSConfig != cw.GetTLSConfig(), "TLS config was not updated after certificate change")
}
