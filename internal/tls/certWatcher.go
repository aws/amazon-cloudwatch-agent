// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package tls

import (
	"context"
	"crypto/tls"
	"errors"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
)

// CertWatcher watches certificate and key files for changes.  When either file
// changes, it reads and parses both and calls an optional callback with the new
// certificate.
type CertWatcher struct {
	sync.RWMutex

	watcher          *fsnotify.Watcher
	logger           *zap.Logger
	currentTLSConfig *tls.Config

	certPath string
	keyPath  string
	caPath   string

	// callback is a function to be invoked when the certificate changes.
	callback func()
}

var NewCertWatcherFunc = NewCertWatcher

// NewCertWatcher returns a new CertWatcher watching the given server certificate and client certificate.
func NewCertWatcher(certPath, keyPath, caPath string, logger *zap.Logger) (*CertWatcher, error) {
	if certPath == "" || keyPath == "" || caPath == "" {
		return nil, errors.New("cert, key, and ca paths are required")
	}
	var err error

	cw := &CertWatcher{
		certPath: certPath,
		keyPath:  keyPath,
		caPath:   caPath,
		logger:   logger,
	}

	cw.logger.Debug("Creating new certificate watcher with", zap.String("cert", certPath), zap.String("key", keyPath), zap.String("ca", caPath))

	// Initial read of certificate and key.
	if err := cw.ReadTlsConfig(); err != nil {
		return nil, err
	}

	cw.watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return cw, nil
}

// RegisterCallback registers a callback to be invoked when the certificate changes.
func (cw *CertWatcher) RegisterCallback(callback func()) {
	cw.callback = callback
}

// GetTLSConfig fetches the currently loaded tls Config, which may be nil.
func (cw *CertWatcher) GetTLSConfig() *tls.Config {
	cw.RLock()
	defer cw.RUnlock()
	return cw.currentTLSConfig
}

func (cw *CertWatcher) ReadTlsConfig() error {
	cw.logger.Debug("Reading TLS certificate")
	serverConfig := &ServerConfig{
		TLSCert:           cw.certPath,
		TLSKey:            cw.keyPath,
		TLSAllowedCACerts: []string{cw.caPath},
	}
	tlsConfig, err := serverConfig.TLSConfig()
	if err != nil {
		cw.logger.Error("failed to read certificate", zap.Error(err))
		return err
	}

	if tlsConfig != cw.currentTLSConfig {
		cw.logger.Debug("TLS certificate changed")
		cw.Lock()
		cw.currentTLSConfig = tlsConfig
		cw.Unlock()

		// If a callback is registered, invoke it with the new certificate.
		if cw.callback != nil {
			go func() {
				cw.logger.Debug("Invoking callback")
				cw.callback()
			}()
		}
	}
	return nil
}

// Start starts the watch on the certificate and key files.
func (cw *CertWatcher) Start(ctx context.Context) error {
	cw.logger.Debug("Starting certificate watcher")
	files := sets.New(cw.certPath, cw.keyPath, cw.caPath)
	{
		var watchErr error
		if err := wait.PollUntilContextTimeout(ctx, 1*time.Second, 10*time.Second, true, func(ctx context.Context) (done bool, err error) {
			for f := range files {
				if err := cw.watcher.Add(f); err != nil {
					watchErr = err
					return false, nil //nolint:nilerr // We want to keep trying.
				}
			}
			files.Clear()
			return true, nil
		}); err != nil {
			cw.logger.Error("failed to add watches", zap.Error(err), zap.Error(watchErr))
			return errors.Join(err, watchErr)
		}
	}

	go cw.Watch()

	cw.logger.Debug("Successfully started certificate watcher")

	// Block until the context is done.
	<-ctx.Done()

	return cw.watcher.Close()
}

// Watch reads events from the watcher's channel and reacts to changes.
func (cw *CertWatcher) Watch() {
	for {
		select {
		case event, ok := <-cw.watcher.Events:
			// Channel is closed.
			if !ok {
				return
			}

			cw.handleEvent(event)

		case err, ok := <-cw.watcher.Errors:
			// Channel is closed.
			if !ok {
				return
			}

			cw.logger.Error("certificate watch error", zap.Error(err))
		}

	}
}

func (cw *CertWatcher) handleEvent(event fsnotify.Event) {
	// Only care about events which may modify the contents of the file.
	if !(isWrite(event) || isRemove(event) || isCreate(event)) {
		return
	}

	cw.logger.Debug("certificate event", zap.Any("event", event))

	// If the file was removed, re-add the watch.
	if isRemove(event) {
		if err := cw.watcher.Add(event.Name); err != nil {
			cw.logger.Error("error re-watching file", zap.Error(err))
		}
	}

	if err := cw.ReadTlsConfig(); err != nil {
		cw.logger.Error("failed to re-read certificate", zap.Error(err))
	}
}

func isWrite(event fsnotify.Event) bool {
	return event.Op.Has(fsnotify.Write)
}

func isCreate(event fsnotify.Event) bool {
	return event.Op.Has(fsnotify.Create)
}

func isRemove(event fsnotify.Event) bool {
	return event.Op.Has(fsnotify.Remove)
}
