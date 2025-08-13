package internal

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff/v4"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opamp-go/client/internal/utils"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/internal"
	"github.com/open-telemetry/opamp-go/protobufs"
)

const (
	OpAMPPlainHTTPMethod     = "POST"
	defaultPollingIntervalMs = 30 * 1000 // default interval is 30 seconds.
)

const (
	headerContentEncoding = "Content-Encoding"
	encodingTypeGZip      = "gzip"
)

type requestWrapper struct {
	*http.Request

	bodyReader func() io.ReadCloser
}

func bodyReader(buf []byte) func() io.ReadCloser {
	return func() io.ReadCloser {
		return io.NopCloser(bytes.NewReader(buf))
	}
}

func (r *requestWrapper) rewind(ctx context.Context) {
	r.Body = r.bodyReader()
	r.Request = r.Request.WithContext(ctx)
}

// HTTPSender allows scheduling messages to send. Once run, it will loop through
// a request/response cycle for each message to send and will process all received
// responses using a receivedProcessor. If there are no pending messages to send
// the HTTPSender will wait for the configured polling interval.
type HTTPSender struct {
	SenderCommon

	url                string
	logger             types.Logger
	client             *http.Client
	callbacks          types.Callbacks
	pollingIntervalMs  int64
	compressionEnabled bool

	// Headers to send with all requests.
	getHeader func() http.Header

	// Processor to handle received messages.
	receiveProcessor receivedProcessor
}

// NewHTTPSender creates a new Sender that uses HTTP to send messages
// with default settings.
func NewHTTPSender(logger types.Logger) *HTTPSender {
	h := &HTTPSender{
		SenderCommon:      NewSenderCommon(),
		logger:            logger,
		client:            utils.NewHttpClient(),
		pollingIntervalMs: defaultPollingIntervalMs,
	}
	// initialize the headers with no additional headers
	h.SetRequestHeader(nil, nil)
	return h
}

// Run starts the processing loop that will perform the HTTP request/response.
// When there are no more messages to send Run will suspend until either there is
// a new message to send or the polling interval elapses.
// Should not be called concurrently with itself. Can be called concurrently with
// modifying NextMessage().
// Run continues until ctx is cancelled.
func (h *HTTPSender) Run(
	ctx context.Context,
	url string,
	callbacks types.Callbacks,
	clientSyncedState *ClientSyncedState,
	packagesStateProvider types.PackagesStateProvider,
	packageSyncMutex *sync.Mutex,
	reporterInterval time.Duration,
) {
	h.url = url
	h.callbacks = callbacks
	h.receiveProcessor = newReceivedProcessor(h.logger, callbacks, h, clientSyncedState, packagesStateProvider, packageSyncMutex, reporterInterval)

	// we need to detect if the redirect was ever set, if not, we want default behaviour
	if callbacks.CheckRedirect != nil {
		h.client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			// viaResp only non-nil for ws client
			return callbacks.CheckRedirect(req, via, nil)
		}
	}

	for {
		pollingTimer := time.NewTimer(time.Millisecond * time.Duration(atomic.LoadInt64(&h.pollingIntervalMs)))
		select {
		case <-h.hasPendingMessage:
			// Have something to send. Stop the polling timer and send what we have.
			pollingTimer.Stop()
			h.makeOneRequestRoundtrip(ctx)

		case <-pollingTimer.C:
			// Polling interval has passed. Force a status update.
			h.NextMessage().Update(func(msg *protobufs.AgentToServer) {})
			// This will make hasPendingMessage channel readable, so we will enter
			// the case above on the next iteration of the loop.
			h.ScheduleSend()

		case <-ctx.Done():
			return
		}
	}
}

// SetRequestHeader sets additional HTTP headers to send with all future requests.
// Should not be called concurrently with any other method.
func (h *HTTPSender) SetRequestHeader(baseHeaders http.Header, headerFunc func(http.Header) http.Header) {
	if baseHeaders == nil {
		baseHeaders = http.Header{}
	}

	if headerFunc == nil {
		headerFunc = func(h http.Header) http.Header {
			return h
		}
	}

	h.getHeader = func() http.Header {
		requestHeader := headerFunc(baseHeaders.Clone())
		requestHeader.Set(headerContentType, contentTypeProtobuf)
		if h.compressionEnabled {
			requestHeader.Set(headerContentEncoding, encodingTypeGZip)
		}

		return requestHeader
	}
}

// makeOneRequestRoundtrip sends a request and receives a response.
// It will retry the request if the server responds with too many
// requests or unavailable status.
func (h *HTTPSender) makeOneRequestRoundtrip(ctx context.Context) {
	resp, err := h.sendRequestWithRetries(ctx)
	if err != nil {
		h.logger.Errorf(ctx, "%v", err)
		return
	}
	if resp == nil {
		// No request was sent and nothing to receive.
		return
	}
	h.receiveResponse(ctx, resp)
}

func (h *HTTPSender) sendRequestWithRetries(ctx context.Context) (*http.Response, error) {
	req, err := h.prepareRequest(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			h.logger.Debugf(ctx, "Client is stopped, will not try anymore.")
		} else {
			h.logger.Errorf(ctx, "Failed prepare request (%v), will not try anymore.", err)
		}
		return nil, err
	}
	if req == nil {
		// Nothing to send.
		return nil, nil
	}

	// Repeatedly try requests with a backoff strategy.
	infiniteBackoff := backoff.NewExponentialBackOff()
	// Make backoff run forever.
	infiniteBackoff.MaxElapsedTime = 0

	interval := time.Duration(0)

	for {
		timer := time.NewTimer(interval)
		interval = infiniteBackoff.NextBackOff()

		select {
		case <-timer.C:
			{
				req.rewind(ctx)
				resp, err := h.client.Do(req.Request)
				if err == nil {
					switch resp.StatusCode {
					case http.StatusOK:
						// We consider it connected if we receive 200 status from the Server.
						h.callbacks.OnConnect(ctx)
						return resp, nil

					case http.StatusTooManyRequests, http.StatusServiceUnavailable:
						interval = recalculateInterval(interval, resp)
						err = fmt.Errorf("server response code=%d", resp.StatusCode)

					default:
						return nil, fmt.Errorf("invalid response from server: %d", resp.StatusCode)
					}
				} else if errors.Is(err, context.Canceled) {
					h.logger.Debugf(ctx, "Client is stopped, will not try anymore.")
					return nil, err
				}

				h.logger.Errorf(ctx, "Failed to do HTTP request (%v), will retry", err)
				h.callbacks.OnConnectFailed(ctx, err)
			}

		case <-ctx.Done():
			h.logger.Debugf(ctx, "Client is stopped, will not try anymore.")
			return nil, ctx.Err()
		}
	}
}

func recalculateInterval(interval time.Duration, resp *http.Response) time.Duration {
	retryAfter := internal.ExtractRetryAfterHeader(resp)
	if retryAfter.Defined && retryAfter.Duration > interval {
		// If the Server suggested connecting later than our interval
		// then honour Server's request, otherwise wait at least
		// as much as we calculated.
		interval = retryAfter.Duration
	}
	return interval
}

func (h *HTTPSender) prepareRequest(ctx context.Context) (*requestWrapper, error) {
	msgToSend := h.nextMessage.PopPending()
	if msgToSend == nil || proto.Equal(msgToSend, &protobufs.AgentToServer{}) {
		// There is no pending message or the message is empty.
		// Nothing to send.
		return nil, nil
	}

	data, err := proto.Marshal(msgToSend)
	if err != nil {
		return nil, err
	}

	r, err := http.NewRequestWithContext(ctx, OpAMPPlainHTTPMethod, h.url, nil)
	if err != nil {
		return nil, err
	}
	req := requestWrapper{Request: r}

	if h.compressionEnabled {
		var buf bytes.Buffer
		g := gzip.NewWriter(&buf)
		if _, err = g.Write(data); err != nil {
			h.logger.Errorf(ctx, "Failed to compress message: %v", err)
			return nil, err
		}
		if err = g.Close(); err != nil {
			h.logger.Errorf(ctx, "Failed to close the writer: %v", err)
			return nil, err
		}
		req.bodyReader = bodyReader(buf.Bytes())
	} else {
		req.bodyReader = bodyReader(data)
	}
	if err != nil {
		return nil, err
	}

	req.Header = h.getHeader()
	return &req, nil
}

func (h *HTTPSender) receiveResponse(ctx context.Context, resp *http.Response) {
	msgBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		_ = resp.Body.Close()
		h.logger.Errorf(ctx, "cannot read response body: %v", err)
		return
	}
	_ = resp.Body.Close()

	var response protobufs.ServerToAgent
	if err := proto.Unmarshal(msgBytes, &response); err != nil {
		h.logger.Errorf(ctx, "cannot unmarshal response: %v", err)
		return
	}

	h.receiveProcessor.ProcessReceivedMessage(ctx, &response)
}

func (h *HTTPSender) SetHeartbeatInterval(duration time.Duration) error {
	if duration <= 0 {
		return errors.New("heartbeat interval for httpclient must be greater than zero")
	}

	if duration != 0 {
		h.SetPollingInterval(duration)
	}

	return nil
}

// SetPollingInterval sets the interval between polling. Has effect starting from the
// next polling cycle.
func (h *HTTPSender) SetPollingInterval(duration time.Duration) {
	atomic.StoreInt64(&h.pollingIntervalMs, duration.Milliseconds())
}

// EnableCompression enables compression for the sender.
// Should not be called concurrently with Run.
func (h *HTTPSender) EnableCompression() {
	h.compressionEnabled = true
}

func (h *HTTPSender) AddTLSConfig(config *tls.Config) {
	if config != nil {
		h.client.Transport = &http.Transport{
			TLSClientConfig: config,
		}
	}
}
