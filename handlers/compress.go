// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package handlers

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go/aws/request"
)

var gzipPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(io.Discard)
	},
}

func NewRequestCompressionHandler(opNames []string) request.NamedHandler {
	return request.NamedHandler{
		Name: "RequestCompressionHandler",
		Fn: func(req *request.Request) {
			match := false
			for _, opName := range opNames {
				if req.Operation.Name == opName {
					match = true
				}
			}

			if !match {
				return
			}

			buf := new(bytes.Buffer)
			g := gzipPool.Get().(*gzip.Writer)
			g.Reset(buf)
			size, err := io.Copy(g, req.GetBody())
			if err != nil {
				log.Printf("I! Error occurred when trying to compress payload for operation %v, uncompressed request is sent, error: %v", req.Operation.Name, err)
				req.ResetBody()
				return
			}
			g.Close()
			compressedSize := int64(buf.Len())

			if size <= compressedSize {
				log.Printf("D! The payload is not compressed. original payload size: %v, compressed payload size: %v.", size, compressedSize)
				req.ResetBody()
				return
			}

			req.SetBufferBody(buf.Bytes())
			gzipPool.Put(g)
			req.HTTPRequest.ContentLength = compressedSize
			req.HTTPRequest.Header.Set("Content-Length", fmt.Sprintf("%d", compressedSize))
			req.HTTPRequest.Header.Set("Content-Encoding", "gzip")
		},
	}
}
