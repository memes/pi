package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
)

// Failed to load CA cert.
var errFailedToAppendCACert = errors.New("failed to append CA cert to CA pool")

// Creates a new pool of x509 certificates from the list of file paths provided,
// appended to any system installed certificates.
func newCACertPool(cacerts []string) (*x509.CertPool, error) {
	logger := logger.V(1).WithValues("cacerts", cacerts)
	if len(cacerts) == 0 {
		logger.V(0).Info("No CA certificate paths provided; returning nil for CA cert pool")
		return nil, nil
	}
	logger.V(0).Info("Building certificate pool from file(s)")
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("failed to build new CA cert pool from SystemCertPool: %w", err)
	}
	for _, cacert := range cacerts {
		ca, err := ioutil.ReadFile(cacert)
		if err != nil {
			return nil, fmt.Errorf("failed to read from certificate file %s: %w", cacert, err)
		}
		if ok := pool.AppendCertsFromPEM(ca); !ok {
			return nil, fmt.Errorf("failed to process CA cert %s: %w", cacert, errFailedToAppendCACert)
		}
	}
	return pool, nil
}

// Creates a new TLS configuration from supplied arguments. If a certificate and
// key are provided, the loaded x509 certificate will be added as the certificate
// to present to remote side of TLS connections. An optional pool of CA certificates
// can be provided as ClientCA and/or RootCA verification.
func newTLSConfig(certFile, keyFile string, clientCAs, rootCAs *x509.CertPool) (*tls.Config, error) {
	logger := logger.V(1).WithValues(TLSCertFlagName, certFile, TLSKeyFlagName, keyFile, "hasClientCAs", clientCAs != nil, "hasRootCAs", rootCAs != nil)
	logger.V(0).Info("Preparing TLS configuration")
	tlsConf := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	if certFile != "" && keyFile != "" {
		logger.V(1).Info("Loading x509 certificate and key")
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate %s and key %s: %w", certFile, keyFile, err)
		}
		tlsConf.Certificates = []tls.Certificate{cert}
	}
	if clientCAs != nil {
		logger.V(1).Info("Add x509 certificate pool to ClientCAs")
		tlsConf.ClientCAs = clientCAs
	}
	if rootCAs != nil {
		logger.V(1).Info("Add x509 certificate pool to RootCAs")
		tlsConf.RootCAs = rootCAs
	}
	return tlsConf, nil
}
