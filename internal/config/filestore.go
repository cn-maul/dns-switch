package config

// FileStore is the default Store implementation backed by the TOML config file.
type FileStore struct{}

func (FileStore) Read() (*Config, error)              { return Read() }
func (FileStore) SaveLastTest(o string, r float64) error { return SaveLastTest(o, r) }
func (FileStore) WriteBackup(backend string) error     { return WriteBackup(backend) }
func (FileStore) ClearBackup() error                   { return ClearBackup() }
func (FileStore) LookupServer(servers map[string]string, name string) (string, bool) {
	return LookupServer(servers, name)
}

// Compile-time check.
var _ Store = FileStore{}
