package tls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// Config holds TLS configuration.
type Config struct {
	// Server certificate and key
	CertFile string
	KeyFile  string

	// CA certificate for verifying client certificates (mTLS)
	CAFile string

	// Client certificate and key (for mTLS client connections)
	ClientCertFile string
	ClientKeyFile  string

	// Client CA certificate (for verifying server certificates)
	ClientCAFile string

	// Whether to require client certificates (mTLS)
	RequireClientCert bool

	// Whether client TLS is optional (some clients may not have certs)
	OptionalClientTLS bool

	// Minimum TLS version
	MinVersion uint16

	// Maximum TLS version
	MaxVersion uint16

	// Cipher suites (if empty, uses default)
	CipherSuites []uint16

	// InsecureSkipVerify (for testing only - DO NOT use in production)
	InsecureSkipVerify bool
}

// DefaultConfig returns a default TLS configuration.
func DefaultConfig() Config {
	return Config{
		RequireClientCert: false,
		OptionalClientTLS: false,
		MinVersion:        tls.VersionTLS12,
		MaxVersion:        tls.VersionTLS13,
	}
}

// ServerConfig creates a TLS configuration for a server.
func (c *Config) ServerConfig() (*tls.Config, error) {
	if c.CertFile == "" || c.KeyFile == "" {
		return nil, fmt.Errorf("server certificate and key files are required")
	}

	// Load server certificate
	cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   c.MinVersion,
		MaxVersion:   c.MaxVersion,
	}

	// Configure client certificate verification (mTLS)
	if c.CAFile != "" {
		caCert, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		config.ClientCAs = caCertPool

		if c.RequireClientCert {
			config.ClientAuth = tls.RequireAndVerifyClientCert
		} else if c.OptionalClientTLS {
			config.ClientAuth = tls.VerifyClientCertIfGiven
		} else {
			config.ClientAuth = tls.RequireAndVerifyClientCert
		}
	} else if c.RequireClientCert {
		return nil, fmt.Errorf("CA file is required when RequireClientCert is true")
	}

	// Set cipher suites if specified
	if len(c.CipherSuites) > 0 {
		config.CipherSuites = c.CipherSuites
	}

	return config, nil
}

// ClientConfig creates a TLS configuration for a client.
func (c *Config) ClientConfig() (*tls.Config, error) {
	config := &tls.Config{
		MinVersion: c.MinVersion,
		MaxVersion: c.MaxVersion,
	}

	// Load client certificate if provided (mTLS)
	if c.ClientCertFile != "" && c.ClientKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.ClientCertFile, c.ClientKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		config.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate for verifying server certificate
	if c.ClientCAFile != "" {
		caCert, err := os.ReadFile(c.ClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read client CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse client CA certificate")
		}

		config.RootCAs = caCertPool
	}

	// Set insecure skip verify (for testing only)
	if c.InsecureSkipVerify {
		config.InsecureSkipVerify = true
	}

	// Set cipher suites if specified
	if len(c.CipherSuites) > 0 {
		config.CipherSuites = c.CipherSuites
	}

	return config, nil
}

// MTLSConfig creates a configuration for mutual TLS (mTLS) node-to-node communication.
func MTLSConfig(certFile, keyFile, caFile string) Config {
	return Config{
		CertFile:          certFile,
		KeyFile:           keyFile,
		CAFile:            caFile,
		ClientCertFile:    certFile, // Same cert for client
		ClientKeyFile:     keyFile,  // Same key for client
		ClientCAFile:      caFile,   // Same CA for client
		RequireClientCert: true,
		OptionalClientTLS: false,
		MinVersion:        tls.VersionTLS12,
		MaxVersion:        tls.VersionTLS13,
	}
}

// ServerOnlyConfig creates a configuration for server-only TLS (no client certs required).
func ServerOnlyConfig(certFile, keyFile string) Config {
	return Config{
		CertFile:          certFile,
		KeyFile:           keyFile,
		RequireClientCert: false,
		OptionalClientTLS: false,
		MinVersion:        tls.VersionTLS12,
		MaxVersion:        tls.VersionTLS13,
	}
}

// OptionalClientTLSConfig creates a configuration with optional client TLS.
func OptionalClientTLSConfig(certFile, keyFile, caFile string) Config {
	return Config{
		CertFile:          certFile,
		KeyFile:           keyFile,
		CAFile:            caFile,
		RequireClientCert: false,
		OptionalClientTLS: true,
		MinVersion:        tls.VersionTLS12,
		MaxVersion:        tls.VersionTLS13,
	}
}

// LoadServerConfig loads a server TLS configuration from file paths.
func LoadServerConfig(certFile, keyFile, caFile string, requireClientCert bool) (*tls.Config, error) {
	config := Config{
		CertFile:          certFile,
		KeyFile:           keyFile,
		CAFile:            caFile,
		RequireClientCert: requireClientCert,
		MinVersion:        tls.VersionTLS12,
		MaxVersion:        tls.VersionTLS13,
	}
	return config.ServerConfig()
}

// LoadClientConfig loads a client TLS configuration from file paths.
func LoadClientConfig(clientCertFile, clientKeyFile, clientCAFile string) (*tls.Config, error) {
	config := Config{
		ClientCertFile: clientCertFile,
		ClientKeyFile:  clientKeyFile,
		ClientCAFile:   clientCAFile,
		MinVersion:     tls.VersionTLS12,
		MaxVersion:     tls.VersionTLS13,
	}
	return config.ClientConfig()
}

// LoadMTLSConfig loads a mutual TLS configuration for node-to-node communication.
func LoadMTLSConfig(certFile, keyFile, caFile string) (*tls.Config, *tls.Config, error) {
	serverConfig, err := LoadServerConfig(certFile, keyFile, caFile, true)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load server config: %w", err)
	}

	clientConfig, err := LoadClientConfig(certFile, keyFile, caFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load client config: %w", err)
	}

	return serverConfig, clientConfig, nil
}

