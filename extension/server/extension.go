// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package server

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"

	"github.com/aws/amazon-cloudwatch-agent/extension/entitystore"
)

type Server struct {
	logger         *zap.Logger
	config         *Config
	server         *http.Server
	jsonMarshaller jsoniter.API
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
	}
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	s.setRouter(router)
	s.server = &http.Server{
		Addr:    config.ListenAddress,
		Handler: router,
	}
	return s
}

func (s *Server) Start(context.Context, component.Host) error {
	s.logger.Info("Starting server ...")
	go func() {
		err := s.server.ListenAndServe()
		if err != nil {
			s.logger.Error("failed to serve and listen", zap.Error(err))
		}
	}()
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down server...")
	return s.server.Shutdown(ctx)
}

func (s *Server) k8sPodToServiceMapHandler(c *gin.Context) {
	podServiceEnvironmentMap := getPodServiceEnvironmentMapping()
	s.jsonHandler(c.Writer, podServiceEnvironmentMap)
}

// Added this for testing purpose
var getPodServiceEnvironmentMapping = func() map[string]entitystore.ServiceEnvironment {
	es := entitystore.GetEntityStore()
	if es != nil && es.GetPodServiceEnvironmentMapping() != nil {
		return es.GetPodServiceEnvironmentMapping()
	}
	return map[string]entitystore.ServiceEnvironment{}
}

func (s *Server) jsonHandler(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	err := s.jsonMarshaller.NewEncoder(w).Encode(data)
	if err != nil {
		s.logger.Error("failed to encode data for http response", zap.Error(err))
	}
}
