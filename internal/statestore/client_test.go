// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statestore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
)

type testHost map[component.ID]component.Component

func (h testHost) GetExtensions() map[component.ID]component.Component {
	return h
}

// recordingStorage is a storage.Extension that records what GetClient was asked for. If err is set,
// GetClient returns it and no client.
type recordingStorage struct {
	component.StartFunc
	component.ShutdownFunc
	kind component.Kind
	id   component.ID
	name string
	err  error
}

func (r *recordingStorage) GetClient(_ context.Context, kind component.Kind, id component.ID, name string) (storage.Client, error) {
	r.kind, r.id, r.name = kind, id, name
	if r.err != nil {
		return nil, r.err
	}
	return newFakeClient(), nil
}

var _ storage.Extension = (*recordingStorage)(nil)

// assertNotConfigError checks that err is not one of the configuration errors of GetClient.
func assertNotConfigError(t *testing.T, err error) {
	t.Helper()
	assert.NotErrorIs(t, err, ErrExtensionNotFound)
	assert.NotErrorIs(t, err, ErrNotStorageExtension)
}

func TestGetClientNilID(t *testing.T) {
	client, err := GetClient(context.Background(), testHost{}, nil, component.KindConnector, component.MustNewID("my_connector"))
	assert.EqualError(t, err, "statestore: no storage extension ID")
	assertNotConfigError(t, err)
	assert.Nil(t, client)
}

func TestGetClientMissingExtension(t *testing.T) {
	storageID := component.MustNewIDWithName("file_storage", "missing")
	client, err := GetClient(context.Background(), testHost{}, &storageID, component.KindConnector, component.MustNewID("my_connector"))
	assert.EqualError(t, err, `statestore: storage extension not found: "file_storage/missing"`)
	assert.ErrorIs(t, err, ErrExtensionNotFound)
	assert.NotErrorIs(t, err, ErrNotStorageExtension)
	assert.Nil(t, client)
}

func TestGetClientNonStorageExtension(t *testing.T) {
	nopFactory := extensiontest.NewNopFactory()
	nop, err := nopFactory.Create(context.Background(), extensiontest.NewNopSettings(nopFactory.Type()), nopFactory.CreateDefaultConfig())
	require.NoError(t, err)
	storageID := component.MustNewID("nop")

	client, err := GetClient(context.Background(), testHost{storageID: nop}, &storageID, component.KindConnector, component.MustNewID("my_connector"))
	assert.EqualError(t, err, `statestore: extension is not a storage extension: "nop"`)
	assert.ErrorIs(t, err, ErrNotStorageExtension)
	assert.NotErrorIs(t, err, ErrExtensionNotFound)
	assert.Nil(t, client)
}

func TestGetClientExtensionError(t *testing.T) {
	extErr := errors.New("open /var/lib/state/connector_signaltoinfometric_pods: permission denied")
	ext := &recordingStorage{err: extErr}
	storageID := component.MustNewID("file_storage")

	client, err := GetClient(context.Background(), testHost{storageID: ext}, &storageID,
		component.KindConnector, component.MustNewIDWithName("signal_to_info_metric", "pods"))
	assert.EqualError(t, err, `statestore: storage extension "file_storage" cannot give a client: `+extErr.Error())
	assert.ErrorIs(t, err, extErr)
	assertNotConfigError(t, err)
	assert.Nil(t, client)
}

// TestGetClientFileStorageCannotOpen removes the directory of a started file_storage extension, so
// that the extension cannot create the file of the client. This is not a configuration error.
func TestGetClientFileStorageCannotOpen(t *testing.T) {
	dir := t.TempDir()
	ext := startFileStorage(t, dir)
	require.NoError(t, os.RemoveAll(dir))
	storageID := component.MustNewID("file_storage")

	client, err := GetClient(context.Background(), testHost{storageID: ext}, &storageID,
		component.KindConnector, component.MustNewIDWithName("signal_to_info_metric", "pods"))
	assert.ErrorContains(t, err, `statestore: storage extension "file_storage" cannot give a client: `)
	assert.ErrorIs(t, err, os.ErrNotExist)
	assertNotConfigError(t, err)
	assert.Nil(t, client)
	require.NoError(t, ext.Shutdown(context.Background()))
}

func TestGetClientPassesKindAndNormalizedID(t *testing.T) {
	ext := &recordingStorage{}
	storageID := component.MustNewID("file_storage")

	client, err := GetClient(context.Background(), testHost{storageID: ext}, &storageID,
		component.KindConnector, component.MustNewIDWithName("signal_to_info_metric", "pods"))
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, component.KindConnector, ext.kind)
	assert.Equal(t, component.MustNewIDWithName("signaltoinfometric", "pods"), ext.id)
	assert.Empty(t, ext.name)
}

func startFileStorage(t *testing.T, dir string) extension.Extension {
	t.Helper()
	f := filestorage.NewFactory()
	cfg := f.CreateDefaultConfig().(*filestorage.Config)
	cfg.Directory = dir
	ext, err := f.Create(context.Background(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))
	return ext
}

// TestFileStorageRoundTrip writes through a real file_storage extension, restarts the extension on
// the same directory, and reads the record back, as happens across an agent restart.
func TestFileStorageRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	storageID := component.MustNewID("file_storage")
	connectorID := component.MustNewIDWithName("signal_to_info_metric", "pods")
	opts := Options{Key: "pods", Version: 1, MaxBytes: 1 << 20, Logger: zap.NewNop()}
	want := record{Pods: map[string]string{"ns/pod-a": "uid-1"}}

	ext := startFileStorage(t, dir)
	client, err := GetClient(ctx, testHost{storageID: ext}, &storageID, component.KindConnector, connectorID)
	require.NoError(t, err)
	writer, err := New[record](client, opts)
	require.NoError(t, err)
	_, found, err := writer.Load(ctx)
	require.NoError(t, err)
	require.False(t, found)
	writer.Start(constSnapshot(want))
	writer.MarkChanged()
	require.NoError(t, writer.Shutdown(ctx), "Shutdown saves the pending change and closes the client")
	require.NoError(t, ext.Shutdown(ctx))
	// The file name shows the extension received the connector kind and the normalized ID.
	assert.FileExists(t, filepath.Join(dir, "connector_signaltoinfometric_pods"))

	ext = startFileStorage(t, dir)
	client, err = GetClient(ctx, testHost{storageID: ext}, &storageID, component.KindConnector, connectorID)
	require.NoError(t, err)
	reader, err := New[record](client, opts)
	require.NoError(t, err)
	got, found, err := reader.Load(ctx)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, want, got)
	require.NoError(t, reader.Shutdown(ctx))
	require.NoError(t, ext.Shutdown(ctx))
}
