package utils

import "net/http"

func NewHttpClient() *http.Client {
	client := &http.Client{}
	// Clone the default transport to avoid being affected by global state changes.
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr = tr.Clone()
		client.Transport = tr
	}
	return client
}
