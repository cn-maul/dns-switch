package config

// Store abstracts configuration persistence for dependency injection.
// The default implementation is FileStore; tests can provide a memory-backed
// implementation without touching the filesystem.
type Store interface {
	Read() (*Config, error)
	SaveLastTest(optimal string, rttMs float64) error
	WriteBackup(backend string) error
	ClearBackup() error
	LookupServer(servers map[string]string, name string) (string, bool)
}
