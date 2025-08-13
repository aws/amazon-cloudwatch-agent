package internal

import (
	"context"
	"testing"
	"time"

	"github.com/open-telemetry/opamp-go/protobufs"
	"github.com/stretchr/testify/require"
)

func Test_DownloadReporter_Report(t *testing.T) {
	ts := time.Now()
	reporter := &downloadReporter{
		start:         ts,
		interval:      time.Millisecond,
		packageLength: 2,
		done:          make(chan struct{}),
	}
	defer reporter.stop()

	// Write before report to avoid timeing issues.
	n, err := reporter.Write([]byte{0})
	require.NoError(t, err)
	require.Equal(t, 1, n)

	ch := make(chan protobufs.PackageDownloadDetails)
	updateFn := func(_ context.Context, d protobufs.PackageDownloadDetails) error {
		ch <- d
		return nil
	}
	reporter.report(context.Background(), updateFn)

	var details protobufs.PackageDownloadDetails
	select {
	case details = <-ch:
	case <-time.After(time.Millisecond * 100):
		t.Fatal("Did not recieve report after 100ms")
	}

	require.Equal(t, 50, int(details.DownloadPercent))
}
