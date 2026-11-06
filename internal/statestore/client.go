// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package statestore

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/xextension/storage"
)

// GetClient wraps these errors when the storage ID does not name a storage extension of the host.
// They are configuration errors. Use errors.Is to test for them.
var (
	// ErrExtensionNotFound means that the host has no extension with the storage ID.
	ErrExtensionNotFound = errors.New("statestore: storage extension not found")
	// ErrNotStorageExtension means that the extension with the storage ID is not a storage extension.
	ErrNotStorageExtension = errors.New("statestore: extension is not a storage extension")
)

// GetClient finds the storage extension storageID in host and returns a client of that extension for
// the component with the given kind and id. It returns an error when storageID is nil. It returns an
// error that wraps ErrExtensionNotFound when host has no extension with that ID, and an error that
// wraps ErrNotStorageExtension when that extension is not a storage extension. Any other error comes
// from the GetClient method of the extension, for example when file_storage cannot open its file.
func GetClient(ctx context.Context, host component.Host, storageID *component.ID, kind component.Kind, id component.ID) (storage.Client, error) {
	if storageID == nil {
		return nil, errors.New("statestore: no storage extension ID")
	}
	ext, found := host.GetExtensions()[*storageID]
	if !found {
		return nil, fmt.Errorf("%w: %q", ErrExtensionNotFound, storageID.String())
	}
	storageExt, isStorage := ext.(storage.Extension)
	if !isStorage {
		return nil, fmt.Errorf("%w: %q", ErrNotStorageExtension, storageID.String())
	}
	// Remove the underscores of the type, so a rename that adds underscores keeps the same storage (https://github.com/open-telemetry/opentelemetry-collector/issues/14988).
	clientID := component.MustNewIDWithName(strings.ReplaceAll(id.Type().String(), "_", ""), id.Name())
	client, err := storageExt.GetClient(ctx, kind, clientID, "")
	if err != nil {
		return nil, fmt.Errorf("statestore: storage extension %q cannot give a client: %w", storageID.String(), err)
	}
	return client, nil
}
