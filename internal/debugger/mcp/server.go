// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package mcp

import (
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
)

func StartMCPServer() {
	fmt.Fprintf(os.Stderr, "Starting MCP server...\n")
	s := server.NewMCPServer(
		"CWAgent Debugger Server",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true),
		server.WithPromptCapabilities(true),
		server.WithRecovery(),
	)

	RegisterAllTools(s)
	fmt.Fprintf(os.Stderr, "Tools registered\n")
	RegisterAllResources(s)
	fmt.Fprintf(os.Stderr, "Resources registered\n")
	RegisterAllPrompts(s)
	fmt.Fprintf(os.Stderr, "Prompts registered\n")
	fmt.Println("Starting server...")

	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}
}
