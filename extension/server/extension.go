// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jellydator/ttlcache/v3"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
)

type Server struct {
	logger         *zap.Logger
	config         *Config
	jsonMarshaller jsoniter.API
	httpsServer    *http.Server
	serverConfig   configtls.ServerConfig
	ctx            context.Context
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

	if s.config.TLSCertPath == "" || s.config.TLSKeyPath == "" || s.config.TLSCAPath == "" {
		s.logger.Error("cert, key, and ca paths are required")
		return s
	}

	s.serverConfig = configtls.ServerConfig{
		Config: configtls.Config{
			CertFile: s.config.TLSCertPath,
			KeyFile:  s.config.TLSKeyPath,
		},
		ClientCAFile:       s.config.TLSCAPath,
		ReloadClientCAFile: true,
	}

	tlsConfig, err := s.serverConfig.LoadTLSConfig(context.Background())
	if err != nil {
		s.logger.Error("failed to load TLS config", zap.Error(err))
		return s
	}

	httpsRouter := gin.New()
	s.setRouter(httpsRouter)

	s.httpsServer = &http.Server{
		Addr:              config.ListenAddress,
		Handler:           httpsRouter,
		ReadHeaderTimeout: 90 * time.Second,
		TLSConfig:         tlsConfig,
	}

	return s
}

func (s *Server) Start(context.Context, component.Host) error {
	if s.httpsServer != nil {
		s.logger.Debug("Starting HTTPS server...")
		go func() {
			err := s.httpsServer.ListenAndServeTLS("", "")
			if err != nil {
				s.logger.Error("failed to serve and listen", zap.Error(err))
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
