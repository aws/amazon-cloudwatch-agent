package server

import (
	"context"
	"net"
	"net/http"
)

type connContextKeyType struct {
	key string
}

var connContextKey = connContextKeyType{key: "httpconn"}

func contextWithConn(ctx context.Context, c net.Conn) context.Context {
	// Create a new context that stores the net.Conn. For use as ConnContext func
	// of http.Server to remember the connection in the context.
	return context.WithValue(ctx, connContextKey, c)
}

func connFromRequest(r *http.Request) net.Conn {
	// Extract the net.Conn from the context of the specified http.Request.
	return r.Context().Value(connContextKey).(net.Conn)
}
