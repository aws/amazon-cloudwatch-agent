package types

import (
	"context"
	"io"

	"github.com/open-telemetry/opamp-go/protobufs"
)

// PackagesSyncer can be used by the Agent to initiate syncing a package from the Server.
// The PackagesSyncer instance knows the right context: the particular OpAMPClient and
// the particular PackageAvailable message the OnPackageAvailable callback was called for.
type PackagesSyncer interface {
	// Sync the available package from the Server to the Agent.
	// The Agent must supply an PackagesStateProvider in StartSettings to let the Sync
	// function know what is available locally, what data needs to be synced and how the
	// data can be stored locally.
	// Sync typically returns immediately and continues working in the background,
	// downloading the packages and applying the changes to the local state.
	// Sync should be called once only.
	Sync(ctx context.Context) error

	// Done returns a channel which is readable when the Sync is complete.
	Done() <-chan struct{}
}

// PackageState represents the state of a package in the Agent's local storage.
type PackageState struct {
	// Exists indicates that the package exists locally. The rest of the fields
	// must be ignored if this field is false.
	Exists bool

	Type    protobufs.PackageType
	Hash    []byte
	Version string
}

// PackagesStateProvider is an interface that is used by PackagesSyncer.Sync() to
// query and update the Agent's local state of packages.
// It is recommended that the local state is stored persistently so that after
// Agent restarts full state syncing is not required.
type PackagesStateProvider interface {
	// AllPackagesHash returns the hash of all packages previously set via SetAllPackagesHash().
	AllPackagesHash() ([]byte, error)

	// SetAllPackagesHash must remember the AllPackagesHash. Must be returned
	// later when AllPackagesHash is called. SetAllPackagesHash is called after all
	// package updates complete successfully.
	SetAllPackagesHash(hash []byte) error

	// Packages returns the names of all packages that exist in the Agent's local storage.
	Packages() ([]string, error)

	// PackageState returns the state of a local package. packageName is one of the names
	// that were returned by Packages().
	// Returns (PackageState{Exists:false},nil) if package does not exist locally.
	PackageState(packageName string) (state PackageState, err error)

	// SetPackageState must remember the state for the specified package. Must be returned
	// later when PackageState is called. SetPackageState is called after UpdateContent
	// call completes successfully.
	// The state.Type must be equal to the current Type of the package otherwise
	// the call may fail with an error.
	SetPackageState(packageName string, state PackageState) error

	// CreatePackage creates the package locally. If the package existed must return an error.
	// If the package did not exist its hash should be set to nil.
	CreatePackage(packageName string, typ protobufs.PackageType) error

	// FileContentHash returns the content hash of the package file that exists locally.
	// Returns (nil,nil) if package or package file is not found.
	FileContentHash(packageName string) ([]byte, error)

	// UpdateContent must create or update the package content file. The entire content
	// of the file must be replaced by the data. The data must be read until
	// it returns an EOF. If reading from data fails UpdateContent must abort and return
	// an error.
	// Content hash must be updated if the data is updated without failure.
	// The function must cancel and return an error if the context is cancelled.
	UpdateContent(ctx context.Context, packageName string, data io.Reader, contentHash, signature []byte) error

	// DeletePackage deletes the package from the Agent's local storage.
	DeletePackage(packageName string) error

	// LastReportedStatuses returns the value previously set via SetLastReportedStatuses.
	LastReportedStatuses() (*protobufs.PackageStatuses, error)

	// SetLastReportedStatuses saves the statuses in the local state. This is called
	// periodically during syncing process to save the most recent statuses.
	// Depending on implementation, this method may be called concurrently if a client
	// downloads many packages at once. Implementors of this interface should take care
	// to ensure that conflicting writes do not occur.
	SetLastReportedStatuses(statuses *protobufs.PackageStatuses) error
}
