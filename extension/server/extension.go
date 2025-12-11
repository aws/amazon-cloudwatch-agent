// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jellydator/ttlcache/v3"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
	tlsInternal "github.com/aws/amazon-cloudwatch-agent/internal/tls"
)

type Server struct {
	logger         *zap.Logger
	config         *Config
	jsonMarshaller jsoniter.API
	httpsServer    *http.Server
	ctx            context.Context
	watcher        *tlsInternal.CertWatcher
}

var _ extension.Extension = (*Server)(nil)

func (s *Server) setRouter(router *gin.Engine) {
	router.Use(gin.Recovery())
	//disabling the gin default behavior of encoding/decoding the request path
	router.UseRawPath = true
	router.UnescapePathValues = false
	router.GET("/kubernetes/pod-to-service-env-map", s.k8sPodToServiceMapHandler)
}

func NewServer(logger *zap.Logger, config *Config) *Server {
	s := &Server{
		logger:         logger,
		config:         config,
		jsonMarshaller: jsoniter.ConfigCompatibleWithStandardLibrary,
		ctx:            context.Background(),
	}
	gin.SetMode(gin.ReleaseMode)

	// Initialize a new cert watcher with cert/key pair
	watcher, err := tlsInternal.NewCertWatcher(config.TLSCertPath, config.TLSKeyPath, config.TLSCAPath, logger)
	if err != nil {
		s.logger.Debug("failed to initialize cert watcher", zap.Error(err))
		return s
	}

	s.watcher = watcher

	watcher.RegisterCallback(func() {
		s.logger.Debug("Calling registered callback, reloading TLS server")
		if err := s.reloadServer(watcher.GetTLSConfig()); err != nil {
			s.logger.Error("Failed to reload TLS server", zap.Error(err))
		}
	})

	// Start goroutine with certwatcher running fsnotify against supplied certdir
	go func() {
		if err := watcher.Start(s.ctx); err != nil {
			s.logger.Error("failed to start cert watcher", zap.Error(err))
			return
		}
	}()

	httpsRouter := gin.New()
	s.setRouter(httpsRouter)

	s.httpsServer = &http.Server{Addr: config.ListenAddress, Handler: httpsRouter, ReadHeaderTimeout: 90 * time.Second, TLSConfig: watcher.GetTLSConfig()}

	return s
}

func (s *Server) Start(context.Context, component.Host) error {
	if s.httpsServer != nil {
		s.logger.Debug("Starting HTTPS server...")
		go func() {
			err := s.httpsServer.ListenAndServeTLS("", "")
			if err != nil {
				s.logger.Debug("failed to serve and listen", zap.Error(err))
			}
		}()
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.ctx.Done()
	if s.httpsServer != nil {
		s.logger.Debug("Shutting down HTTPS server...")
		return s.httpsServer.Shutdown(ctx)
	}
	return nil
}

func (s *Server) reloadServer(config *tls.Config) error {
	s.logger.Debug("Reloading TLS Server...")
	// close the current server
	if s.httpsServer != nil {
		// closing the server gracefully
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		// do not use Close plus wait for Shutdown to return, otherwise ListenAndServe returns ErrServerClosed
		if err := s.httpsServer.Shutdown(ctx); err != nil {
			s.logger.Error("Failed to shutdown HTTPS server", zap.Error(err))
		}
	}
	// Create a new HTTP server with the new router and updated TLS config
	httpsRouter := gin.New()
	s.setRouter(httpsRouter)
	s.httpsServer = &http.Server{
		Addr:              s.config.ListenAddress,
		Handler:           httpsRouter,
		TLSConfig:         config,
		ReadHeaderTimeout: 90 * time.Second,
	}

	go func() {
		err := s.httpsServer.ListenAndServeTLS("", "")
		if err != nil {
			s.logger.Error("failed to serve and listen", zap.Error(err))
		}
	}()
	return nil
}

func (s *Server) k8sPodToServiceMapHandler(c *gin.Context) {
	podServiceEnvironmentMap := convertTtlCacheToMap(getPodServiceEnvironmentMapping())
	s.jsonHandler(c.Writer, podServiceEnvironmentMap)
}

// Added this for testing purpose
var getPodServiceEnvironmentMapping = func() *ttlcache.Cache[string, entitystore.ServiceEnvironment] {
	es := entitystore.GetEntityStore()
	if es != nil && es.GetPodServiceEnvironmentMapping() != nil {
		return es.GetPodServiceEnvironmentMapping()
	}
	return ttlcache.New[string, entitystore.ServiceEnvironment](
		ttlcache.WithTTL[string, entitystore.ServiceEnvironment](time.Hour * 1),
	)
}

func (s *Server) jsonHandler(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	err := s.jsonMarshaller.NewEncoder(w).Encode(data)
	if err != nil {
		s.logger.Error("failed to encode data for http response", zap.Error(err))
	}
}

func convertTtlCacheToMap(cache *ttlcache.Cache[string, entitystore.ServiceEnvironment]) map[string]entitystore.ServiceEnvironment {
	m := make(map[string]entitystore.ServiceEnvironment)
	for pod, se := range cache.Items() {
		m[pod] = se.Value()
	}
	return m
}
