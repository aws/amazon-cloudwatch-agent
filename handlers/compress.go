package handlers

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
)

const (
	// Magic number that only start trying gzip compression when payload is larger
	// https://webmasters.stackexchange.com/questions/31750/what-is-recommended-minimum-object-size-for-gzip-performance-benefits
	startCompressionSize = 350
)

var gzipPool = sync.Pool{
	New: func() interface{} {
		return gzip.NewWriter(ioutil.Discard)
	},
}

func NewRequestCompressionHandler(opNames []string) request.NamedHandler {
	return request.NamedHandler{
		Name: "RequestCompressionHandler",
		Fn: func(req *request.Request) {
			match := false
			for _, opName := range opNames {
				if req.Operation.Name != opName {
					match = true
				}
			}
			if !match {
				return
			}

			body := req.GetBody()
			size, err := aws.SeekerLen(body)
			if err == nil && size < startCompressionSize {
				return
			}

			buf := new(bytes.Buffer)
			g := gzipPool.Get().(*gzip.Writer)
			g.Reset(buf)
			size, err = io.Copy(g, body)
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
