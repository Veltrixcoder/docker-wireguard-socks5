package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/caarlos0/env"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun/netstack"
)

type params struct {
	User     string `env:"PROXY_USER" envDefault:""`
	Password string `env:"PROXY_PASS" envDefault:""`
	Port     string `env:"PORT" envDefault:"8080"`
	// WireGuard Params
	WgPrivateKey    string `env:"WIREGUARD_INTERFACE_PRIVATE_KEY"`
	WgAddress       string `env:"WIREGUARD_INTERFACE_ADDRESS"` // e.g., 10.0.0.2/32
	WgPeerPublicKey string `env:"WIREGUARD_PEER_PUBLIC_KEY"`
	WgPeerEndpoint  string `env:"WIREGUARD_PEER_ENDPOINT"`     // e.g., 1.2.3.4:51820
	WgDNS           string `env:"WIREGUARD_INTERFACE_DNS" envDefault:"1.1.1.1"`
}

var tnet *netstack.Net

func handleTunneling(w http.ResponseWriter, r *http.Request) {
	dest := r.URL.Host
	if dest == "" {
		dest = r.Host
	}

	var dest_conn net.Conn
	var err error

	if tnet == nil {
		dest_conn, err = net.DialTimeout("tcp", dest, 10*time.Second)
	} else {
		// Use tnet.Dial to connect through WireGuard
		dest_conn, err = tnet.Dial("tcp", dest)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	client_conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return // Return early if hijack failed
	}
	go transfer(dest_conn, client_conn)
	go transfer(client_conn, dest_conn)
}

func transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}

func handleHTTP(w http.ResponseWriter, r *http.Request) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	
	if tnet != nil {
		// Use tnet.DialContext for HTTP requests
		transport.DialContext = tnet.DialContext
	}

	resp, err := transport.RoundTrip(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func startWireGuard(cfg params) error {
	if cfg.WgPrivateKey == "" || cfg.WgPeerEndpoint == "" {
		log.Println("WireGuard config missing, running in DIRECT mode")
		return nil
	}

	log.Println("Initializing Userspace WireGuard...")

	localIPs := []netip.Addr{}
	if cfg.WgAddress != "" {
		// Handle CIDR notation if present (e.g., 10.0.0.2/32)
		addrStr := strings.Split(cfg.WgAddress, "/")[0]
		addr, err := netip.ParseAddr(addrStr)
		if err == nil {
			localIPs = append(localIPs, addr)
		} else {
			log.Printf("Failed to parse local IP: %v", err)
		}
	}
	
	dnsIP, _ := netip.ParseAddr(cfg.WgDNS)

	tun, tnetErr := netstack.CreateNetTUN(
		localIPs,
		[]netip.Addr{dnsIP},
		1420,
	)
	if tnetErr != nil {
		return fmt.Errorf("failed to create TUN: %w", tnetErr)
	}
	tnet = tun

	dev := device.NewDevice(tun, conn.NewDefaultBind(), device.NewLogger(device.LogLevelVerbose, ""))
	
	uapi := fmt.Sprintf(`private_key=%s
public_key=%s
endpoint=%s
allowed_ip=0.0.0.0/0
`, cfg.WgPrivateKey, cfg.WgPeerPublicKey, cfg.WgPeerEndpoint)

	if err := dev.IpcSet(uapi); err != nil {
		return fmt.Errorf("failed to configure device: %w", err)
	}
	
	if err := dev.Up(); err != nil {
		return fmt.Errorf("failed to bring up device: %w", err)
	}

	log.Println("WireGuard interface is UP")
	return nil
}

func main() {
	cfg := params{}
	if err := env.Parse(&cfg); err != nil {
		log.Printf("Config parse warning: %+v\n", err)
	}

	if err := startWireGuard(cfg); err != nil {
		log.Fatalf("FATAL: Failed to start WireGuard: %v", err)
	}

	log.Printf("Start listening http proxy server on port %s\n", cfg.Port)

	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.User != "" && cfg.Password != "" {
				user, pass, ok := r.BasicAuth()
				if !ok || user != cfg.User || pass != cfg.Password {
					w.Header().Set("Proxy-Authenticate", `Basic realm="Proxy"`)
					http.Error(w, "Unauthorized", http.StatusProxyAuthRequired)
					return
				}
			}
			
			if r.Method == http.MethodConnect {
				handleTunneling(w, r)
			} else if r.URL.Path == "/" {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Proxy Running via Userspace WireGuard"))
			} else {
				handleHTTP(w, r)
			}
		}),
	}
	
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
