package internal

import (
	"context"
	"io"
	"sync"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
)

// InMemPackagesStore is a package store used for testing. Keeps the packages in memory.
type InMemPackagesStore struct {
	allPackagesHash []byte
	pkgState        map[string]types.PackageState
	fileContents    map[string][]byte
	fileSignatures  map[string][]byte
	fileHashes      map[string][]byte

	lastReportedStatuses *protobufs.PackageStatuses
	statusLock           sync.RWMutex

	onAllPackagesHash func()
}

var _ types.PackagesStateProvider = (*InMemPackagesStore)(nil)

func NewInMemPackagesStore() *InMemPackagesStore {
	return &InMemPackagesStore{
		fileContents:   map[string][]byte{},
		fileSignatures: map[string][]byte{},
		fileHashes:     map[string][]byte{},
		pkgState:       map[string]types.PackageState{},
	}
}

func (l *InMemPackagesStore) AllPackagesHash() ([]byte, error) {
	if l.onAllPackagesHash != nil {
		l.onAllPackagesHash()
	}
	return l.allPackagesHash, nil
}

func (l *InMemPackagesStore) Packages() ([]string, error) {
	var names []string
	for k := range l.pkgState {
		names = append(names, k)
	}
	return names, nil
}

func (l *InMemPackagesStore) PackageState(packageName string) (state types.PackageState, err error) {
	if pkg, ok := l.pkgState[packageName]; ok {
		return pkg, nil
	}
	return types.PackageState{Exists: false}, nil
}

func (l *InMemPackagesStore) CreatePackage(packageName string, typ protobufs.PackageType) error {
	l.pkgState[packageName] = types.PackageState{
		Exists: true,
		Type:   typ,
	}
	return nil
}

func (l *InMemPackagesStore) FileContentHash(packageName string) ([]byte, error) {
	return l.fileHashes[packageName], nil
}

func (l *InMemPackagesStore) UpdateContent(_ context.Context, packageName string, data io.Reader, contentHash, signature []byte) error {
	b, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	l.fileContents[packageName] = b
	l.fileSignatures[packageName] = signature
	l.fileHashes[packageName] = contentHash
	return nil
}

func (l *InMemPackagesStore) SetPackageState(packageName string, state types.PackageState) error {
	l.pkgState[packageName] = state
	return nil
}

func (l *InMemPackagesStore) DeletePackage(packageName string) error {
	delete(l.pkgState, packageName)
	return nil
}

func (l *InMemPackagesStore) SetAllPackagesHash(hash []byte) error {
	l.allPackagesHash = hash
	return nil
}

func (l *InMemPackagesStore) GetContent() map[string][]byte {
	return l.fileContents
}

func (l *InMemPackagesStore) GetSignature() map[string][]byte {
	return l.fileSignatures
}

func (l *InMemPackagesStore) LastReportedStatuses() (*protobufs.PackageStatuses, error) {
	l.statusLock.RLock()
	defer l.statusLock.RUnlock()
	return l.lastReportedStatuses, nil
}

func (l *InMemPackagesStore) SetLastReportedStatuses(statuses *protobufs.PackageStatuses) error {
	l.statusLock.Lock()
	defer l.statusLock.Unlock()
	l.lastReportedStatuses = statuses
	return nil
}
