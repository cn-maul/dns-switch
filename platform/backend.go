package platform

// Backend abstracts OS-specific DNS management.
type Backend interface {
	// SetDNS sets one or more DNS servers on the given interface.
	// The first is primary, subsequent are secondary/tertiary.
	SetDNS(iface string, dns ...string) error

	// RestoreDNS restores the interface to its original DNS (DHCP).
	RestoreDNS(iface string) error

	// DefaultIface returns the name of the default network interface.
	DefaultIface() (string, error)

	// Name returns a human-readable name for this backend.
	Name() string
}

// IsPrivilegedError checks whether an error is due to missing system privileges.
func IsPrivilegedError(err error) bool {
	return isPrivilegedError(err)
}
