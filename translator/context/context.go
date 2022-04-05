// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"log"
	"os"

	"github.com/aws/amazon-cloudwatch-agent/translator/config"
)

const (
	TmpFileSuffix = ".tmp"
)

var ctx *Context

func CurrentContext() *Context {
	if ctx == nil {
		ctx = &Context{
			credentials:         make(map[string]string),
			proxy:               make(map[string]string),
			cloudWatchLogConfig: make(map[string]interface{}),
			runInContainer:      os.Getenv(config.RUN_IN_CONTAINER) == config.RUN_IN_CONTAINER_TRUE,
		}
	}
	return ctx
}

//Testing only
func ResetContext() {
	ctx = nil
}

type Context struct {
	os                  string
	inputJsonFilePath   string
	inputJsonDirPath    string
	multiConfig         string
	outputTomlFilePath  string
	mode                string
	credentials         map[string]string
	proxy               map[string]string
	ssl                 map[string]string
	cloudWatchLogConfig map[string]interface{}
	runInContainer      bool
}

func (ctx *Context) Os() string {
	return ctx.os
}

func (ctx *Context) SetOs(os string) {
	ctx.os = config.ToValidOs(os)
}

func (ctx *Context) InputJsonFilePath() string {
	return ctx.inputJsonFilePath
}

func (ctx *Context) SetInputJsonFilePath(inputJsonFilePath string) {
	ctx.inputJsonFilePath = inputJsonFilePath
}

func (ctx *Context) InputJsonDirPath() string {
	return ctx.inputJsonDirPath
}

func (ctx *Context) SetInputJsonDirPath(inputJsonDirPath string) {
	ctx.inputJsonDirPath = inputJsonDirPath
}

func (ctx *Context) MultiConfig() string {
	return ctx.multiConfig
}

func (ctx *Context) SetMultiConfig(multiConfig string) {
	ctx.multiConfig = multiConfig
}

func (ctx *Context) OutputTomlFilePath() string {
	return ctx.outputTomlFilePath
}

func (ctx *Context) SetOutputTomlFilePath(outputTomlFilePath string) {
	ctx.outputTomlFilePath = outputTomlFilePath
}

func (ctx *Context) Mode() string {
	if ctx.mode == "" {
		ctx.mode = config.ModeEC2
	}
	return ctx.mode
}

func (ctx *Context) Credentials() map[string]string {
	return ctx.credentials
}

func (ctx *Context) SSL() map[string]string {
	return ctx.ssl
}

func (ctx *Context) Proxy() map[string]string {
	return ctx.proxy
}

func (ctx *Context) SetMode(mode string) {
	switch mode {
	case config.ModeEC2:
		ctx.mode = config.ModeEC2
	case config.ModeOnPrem:
		ctx.mode = config.ModeOnPrem
	default:
		log.Panicf("Invalid mode %s. Valid mode values are %s and %s.", mode, config.ModeEC2, config.ModeOnPrem)
	}
}

func (ctx *Context) SetCredentials(creds map[string]string) {
	ctx.credentials = creds
}

func (ctx *Context) SetSSL(ssl map[string]string) {
	ctx.ssl = ssl
}

func (ctx *Context) SetProxy(proxy map[string]string) {
	ctx.proxy = proxy
}

func (ctx *Context) SetCloudWatchLogConfig(config map[string]interface{}) {
	ctx.cloudWatchLogConfig = config
}

func (ctx *Context) CloudWatchLogConfig() map[string]interface{} {
	return ctx.cloudWatchLogConfig
}

func (ctx *Context) RunInContainer() bool {
	return ctx.runInContainer
}

func (ctx *Context) SetRunInContainer(runInContainer bool) {
	ctx.runInContainer = runInContainer
}
