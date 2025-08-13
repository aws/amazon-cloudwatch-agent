package internal

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
)

const downloadReporterDefaultInterval = time.Second * 10

type downloadReporter struct {
	start         time.Time
	interval      time.Duration
	packageLength float64

	downloaded atomic.Uint64

	done chan struct{}
}

func newDownloadReporter(interval time.Duration, length int) *downloadReporter {
	if interval <= 0 {
		interval = downloadReporterDefaultInterval
	}
	return &downloadReporter{
		start:         time.Now(),
		interval:      interval,
		packageLength: float64(length),
		done:          make(chan struct{}),
	}
}

// Write tracks the number of bytes downloaded. It will never return an error.
func (p *downloadReporter) Write(b []byte) (int, error) {
	n := len(b)
	p.downloaded.Add(uint64(n))
	return n, nil
}

// report periodically calls the passed function to with the download percent and rate to update the status of a package.
func (p *downloadReporter) report(ctx context.Context, updateFn func(context.Context, protobufs.PackageDownloadDetails) error) {
	go func() {
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.done:
				return
			case <-ticker.C:
				downloadTime := time.Since(p.start)
				downloaded := float64(p.downloaded.Load())
				bps := downloaded / float64(downloadTime/time.Second)
				var downloadPercent float64
				if p.packageLength > 0 {
					downloadPercent = downloaded / p.packageLength * 100
				}
				_ = updateFn(ctx, protobufs.PackageDownloadDetails{
					DownloadPercent:        downloadPercent,
					DownloadBytesPerSecond: bps,
				})
			}
		}
	}()
}

// stop the downloadReporter report goroutine
func (p *downloadReporter) stop() {
	close(p.done)
}
