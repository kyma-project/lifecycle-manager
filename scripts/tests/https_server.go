package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"time"
)

const headerTimeout = 10 * time.Second

func main() {
	directory := flag.String("dir", ".", "Directory to serve files from")
	certFile := flag.String("certfile", "cert.crt", "SSL certificate file")
	keyFile := flag.String("keyfile", "key.crt", "SSL key file")
	port := flag.String("port", "8080", "Port to run the server on")
	flag.Parse()

	mux := http.NewServeMux()
	fs := http.FileServer(http.Dir(*directory))
	mux.Handle("/", fs)

	addr := ":" + *port
	httpsServer := &http.Server{
		Addr:    addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		ReadHeaderTimeout: headerTimeout,
	}

	err := httpsServer.ListenAndServeTLS(*certFile, *keyFile)
	if err != nil {
		log.Fatal("failed to setup http server:", err)
	}
}
