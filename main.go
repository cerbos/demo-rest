// Copyright 2021 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/cerbos/demo-rest/service"
)

func main() {
	listenAddr := flag.String("listen", ":9999", "Address to listen on")
	certFile := flag.String("tlscert", "", "TLS certificate")
	keyFile := flag.String("tlskey", "", "TLS Key")
	cerbosAddr := flag.String("cerbos", "localhost:3593", "Address of the Cerbos server")
	flag.Parse()

	// Create the service
	svc, err := service.New(*cerbosAddr)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	srv := &http.Server{
		Addr:    *listenAddr,
		Handler: svc.Handler(),
	}

	log.Printf("Listening on %s", *listenAddr)

	if *certFile != "" && *keyFile != "" {
		srv.TLSConfig = &tls.Config{
			MinVersion:               tls.VersionTLS13,
			PreferServerCipherSuites: true,
			NextProtos:               []string{"h2"},
		}

		go func() {
			if err := srv.ListenAndServeTLS(*certFile, *keyFile); err != http.ErrServerClosed {
				panic(err)
			}
		}()
	} else {
		log.Printf("WARNING: HTTP server is insecure")
		go func() {
			if err := srv.ListenAndServe(); err != http.ErrServerClosed {
				panic(err)
			}
		}()
	}

	ctx, stopFunc := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stopFunc()

	<-ctx.Done()
	srv.Shutdown(context.Background())
}
