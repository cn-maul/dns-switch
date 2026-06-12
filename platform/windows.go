//go:build windows

package platform

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// WindowsBackend manages DNS on Windows via registry.
type WindowsBackend struct {
	ifaceGUID string
}

// Name returns a human-readable name for this backend.
func (b *WindowsBackend) Name() string {
	return "Windows"
}

// Detect returns the Windows backend.
func Detect() Backend {
	return &WindowsBackend{}
}

// DefaultIface returns the GUID of the default interface.
func (b *WindowsBackend) DefaultIface() (string, error) {
	if b.ifaceGUID != "" {
		return b.ifaceGUID, nil
	}

	guid, err := getDefaultInterfaceGUID()
	if err != nil {
		return "", err
	}
	b.ifaceGUID = guid
	return guid, nil
}

// SetDNS writes one or more DNS servers to the interface's registry key.
// Multiple servers are comma-separated per Windows convention.
func (b *WindowsBackend) SetDNS(iface string, dns ...string) error {
	val := strings.Join(dns, ",")
	return setRegistryDNS(iface, val)
}

// RestoreDNS deletes the NameServer registry value, reverting to DHCP.
func (b *WindowsBackend) RestoreDNS(iface string) error {
	return deleteRegistryDNS(iface)
}

// getDefaultInterfaceGUID finds the network adapter GUID for the
// interface with the lowest metric for the default route.
func getDefaultInterfaceGUID() (string, error) {
	var bestIdx uint32
	sa := &windows.SockaddrInet4{Addr: [4]byte{0, 0, 0, 0}}
	if err := windows.GetBestInterfaceEx(sa, &bestIdx); err != nil {
		return "", fmt.Errorf("GetBestInterface: %w", err)
	}

	adapterList, err := getAdapters()
	if err != nil {
		return "", fmt.Errorf("get adapters: %w", err)
	}

	for _, a := range adapterList {
		if a.IfIndex == int(bestIdx) {
			return a.GUID, nil
		}
	}

	return "", fmt.Errorf("no adapter found for interface index %d", bestIdx)
}

type adapterInfo struct {
	GUID    string
	IfIndex int
}

func getAdapters() ([]adapterInfo, error) {
	const family = windows.AF_INET
	const flags = windows.GAA_FLAG_INCLUDE_PREFIX

	buf := make([]byte, 15*1024)
	var outBufLen uint32 = uint32(len(buf))

	err := windows.GetAdaptersAddresses(family, flags, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &outBufLen)
	if err == windows.ERROR_BUFFER_OVERFLOW {
		buf = make([]byte, outBufLen)
		err = windows.GetAdaptersAddresses(family, flags, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &outBufLen)
	}
	if err != nil {
		return nil, fmt.Errorf("GetAdaptersAddresses: %w", err)
	}

	var adapters []adapterInfo
	addr := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0]))
	for ; addr != nil; addr = addr.Next {
		guid := windows.BytePtrToString(addr.AdapterName)
		adapters = append(adapters, adapterInfo{
			GUID:    guid,
			IfIndex: int(addr.IfIndex),
		})
	}
	return adapters, nil
}

// setRegistryDNS writes a DNS server to the interface's registry key.
func setRegistryDNS(ifaceGUID string, dns string) error {
	keyPath := fmt.Sprintf(
		`SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\%s`,
		ifaceGUID,
	)
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue("NameServer", dns); err != nil {
		return fmt.Errorf("set NameServer: %w", err)
	}
	return nil
}

// deleteRegistryDNS removes the NameServer value, reverting to DHCP.
func deleteRegistryDNS(ifaceGUID string) error {
	keyPath := fmt.Sprintf(
		`SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\%s`,
		ifaceGUID,
	)
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open registry key: %w", err)
	}
	defer k.Close()

	if err := k.DeleteValue("NameServer"); err != nil {
		if err == registry.ErrNotExist {
			return nil // already DHCP
		}
		return fmt.Errorf("delete NameServer: %w", err)
	}
	return nil
}

// isPrivilegedError checks if an error is due to missing admin rights.
func isPrivilegedError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "Access is denied") ||
		strings.Contains(msg, "required privilege")
}

// Ensure WindowsBackend implements Backend.
var _ Backend = (*WindowsBackend)(nil)

