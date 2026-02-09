package main

import (
	"io"
	"log"
	"net"
	"net/http"

	"os/exec"
	"time"

	"github.com/caarlos0/env"
)

type params struct {
	User     string `env:"PROXY_USER" envDefault:""`
	Password string `env:"PROXY_PASS" envDefault:""`
	Port     string `env:"PORT" envDefault:"8080"`
	Up       string `env:"PROXY_UP"   envDefault:""`
}

func handleTunneling(w http.ResponseWriter, r *http.Request) {
	// Use r.URL.Host (from CONNECT line) instead of r.Host (Host header)
	// This allows spoofing the Host header to pass through reverse proxies.
	dest := r.URL.Host
	if dest == "" {
		dest = r.Host
	}
	dest_conn, err := net.DialTimeout("tcp", dest, 10*time.Second)
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
	resp, err := http.DefaultTransport.RoundTrip(r)
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

func main() {
	// Working with app params
	cfg := params{}
	err := env.Parse(&cfg)
	if err != nil {
		log.Printf("%+v\n", err)
	}

	log.Printf("Start listening http proxy service on port %s\n", cfg.Port)

	if cfg.Up != "" {
		err = exec.Command(cfg.Up).Start()
		if err != nil {
			log.Fatal(err)
		}
	}

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
				w.Write([]byte("Proxy Running"))
			} else {
				handleHTTP(w, r)
			}
		}),
	}
	
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
