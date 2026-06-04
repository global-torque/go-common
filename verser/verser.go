//nolint:gochecknoglobals
package verser

import "sync"

var (
	mu         sync.RWMutex
	service    string
	version    string
	repository string
	revisionID string
)

// SetServiceVersionRepositoryRevision sets service version metadata.
func SetServiceVersionRepositoryRevision(serv, ver, repo, rev string) {
	mu.Lock()
	defer mu.Unlock()

	version = ver
	service = serv
	repository = repo
	revisionID = rev
}

// SetServiVersRepoRevis is deprecated. Use SetServiceVersionRepositoryRevision.
func SetServiVersRepoRevis(serv, ver, repo, rev string) {
	SetServiceVersionRepositoryRevision(serv, ver, repo, rev)
}

func GetVersion() string {
	mu.RLock()
	defer mu.RUnlock()

	return version
}

func GetService() string {
	mu.RLock()
	defer mu.RUnlock()

	return service
}

func GetRepository() string {
	mu.RLock()
	defer mu.RUnlock()

	return repository
}

func GetRevisionID() string {
	mu.RLock()
	defer mu.RUnlock()

	return revisionID
}
