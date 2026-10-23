package panel

import (
	"net"
	"os"
	"strings"
)

// certificateNames covers local access plus the configured public address, so
// the emergency self-signed certificate is usable both through localhost and
// through the address clients actually use.
func certificateNames(publicAddress string) (dnsNames []string, ipAddresses []net.IP) {
	dnsSeen := map[string]bool{}
	ipSeen := map[string]bool{}

	addDNS := func(name string) {
		name = strings.TrimSuffix(strings.TrimSpace(name), ".")
		if name == "" || dnsSeen[strings.ToLower(name)] {
			return
		}
		dnsSeen[strings.ToLower(name)] = true
		dnsNames = append(dnsNames, name)
	}
	addIP := func(ip net.IP) {
		if ip == nil {
			return
		}
		key := ip.String()
		if ipSeen[key] {
			return
		}
		ipSeen[key] = true
		ipAddresses = append(ipAddresses, ip)
	}

	addDNS("localhost")
	if hostname, err := os.Hostname(); err == nil {
		addDNS(hostname)
	}
	if publicAddress != "" {
		host := publicHost(publicAddress)
		if ip := net.ParseIP(host); ip != nil {
			addIP(ip)
		} else {
			addDNS(host)
		}
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		addIP(net.ParseIP("127.0.0.1"))
		addIP(net.ParseIP("::1"))
		return dnsNames, ipAddresses
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch value := addr.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			addIP(ip)
		}
	}
	return dnsNames, ipAddresses
}

func publicHost(address string) string {
	if host, _, err := net.SplitHostPort(address); err == nil {
		return host
	}
	return address
}
