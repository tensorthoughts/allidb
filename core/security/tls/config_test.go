package tls

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTestCert creates a test certificate and key file.
func createTestCert(t *testing.T, certFile, keyFile string, isCA bool) {
	// Generate private key
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate private key: %v", err)
	}

	// Create certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Test Org"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{"Test City"},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	if isCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	// Add IP and DNS
	template.IPAddresses = []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback}
	template.DNSNames = []string{"localhost"}

	// Create certificate
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	// Write certificate
	certOut, err := os.Create(certFile)
	if err != nil {
		t.Fatalf("Failed to open cert file: %v", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		t.Fatalf("Failed to write cert: %v", err)
	}

	// Write private key
	keyOut, err := os.Create(keyFile)
	if err != nil {
		t.Fatalf("Failed to open key file: %v", err)
	}
	defer keyOut.Close()

	privDER := x509.MarshalPKCS1PrivateKey(priv)
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER}); err != nil {
		t.Fatalf("Failed to write key: %v", err)
	}
}

func setupTestCerts(t *testing.T) (string, string, string) {
	dir := filepath.Join(os.TempDir(), "tls_test_"+t.Name())
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	caFile := filepath.Join(dir, "ca.pem")

	// Create CA certificate
	createTestCert(t, caFile, filepath.Join(dir, "ca-key.pem"), true)

	// Create server/client certificate
	createTestCert(t, certFile, keyFile, false)

	return certFile, keyFile, caFile
}

func cleanupTestCerts(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	if err != nil {
		t.Logf("Failed to cleanup test directory: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.RequireClientCert {
		t.Error("Expected RequireClientCert false by default")
	}
	if config.MinVersion != tls.VersionTLS12 {
		t.Errorf("Expected MinVersion TLS 1.2, got %d", config.MinVersion)
	}
	if config.MaxVersion != tls.VersionTLS13 {
		t.Errorf("Expected MaxVersion TLS 1.3, got %d", config.MaxVersion)
	}
}

func TestConfig_ServerConfig_NoCert(t *testing.T) {
	config := DefaultConfig()
	// No cert files set

	_, err := config.ServerConfig()
	if err == nil {
		t.Fatal("Expected error when cert files not provided")
	}
}

func TestConfig_ServerConfig_ValidCert(t *testing.T) {
	certFile, keyFile, _ := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := DefaultConfig()
	config.CertFile = certFile
	config.KeyFile = keyFile

	tlsConfig, err := config.ServerConfig()
	if err != nil {
		t.Fatalf("Failed to create server config: %v", err)
	}

	if len(tlsConfig.Certificates) == 0 {
		t.Error("Expected at least one certificate")
	}
}

func TestConfig_ServerConfig_WithCA(t *testing.T) {
	certFile, keyFile, caFile := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := DefaultConfig()
	config.CertFile = certFile
	config.KeyFile = keyFile
	config.CAFile = caFile
	config.RequireClientCert = true

	tlsConfig, err := config.ServerConfig()
	if err != nil {
		t.Fatalf("Failed to create server config: %v", err)
	}

	if tlsConfig.ClientCAs == nil {
		t.Error("Expected ClientCAs to be set")
	}
	if tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("Expected ClientAuth RequireAndVerifyClientCert, got %d", tlsConfig.ClientAuth)
	}
}

func TestConfig_ServerConfig_OptionalClientTLS(t *testing.T) {
	certFile, keyFile, caFile := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := DefaultConfig()
	config.CertFile = certFile
	config.KeyFile = keyFile
	config.CAFile = caFile
	config.OptionalClientTLS = true

	tlsConfig, err := config.ServerConfig()
	if err != nil {
		t.Fatalf("Failed to create server config: %v", err)
	}

	if tlsConfig.ClientAuth != tls.VerifyClientCertIfGiven {
		t.Errorf("Expected ClientAuth VerifyClientCertIfGiven, got %d", tlsConfig.ClientAuth)
	}
}

func TestConfig_ServerConfig_RequireClientCertWithoutCA(t *testing.T) {
	certFile, keyFile, _ := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := DefaultConfig()
	config.CertFile = certFile
	config.KeyFile = keyFile
	config.RequireClientCert = true
	// No CA file

	_, err := config.ServerConfig()
	if err == nil {
		t.Fatal("Expected error when RequireClientCert is true but CA file is missing")
	}
}

func TestConfig_ClientConfig_NoCert(t *testing.T) {
	config := DefaultConfig()

	tlsConfig, err := config.ClientConfig()
	if err != nil {
		t.Fatalf("Failed to create client config: %v", err)
	}

	if len(tlsConfig.Certificates) > 0 {
		t.Error("Expected no client certificates when not provided")
	}
}

func TestConfig_ClientConfig_WithCert(t *testing.T) {
	certFile, keyFile, _ := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := DefaultConfig()
	config.ClientCertFile = certFile
	config.ClientKeyFile = keyFile

	tlsConfig, err := config.ClientConfig()
	if err != nil {
		t.Fatalf("Failed to create client config: %v", err)
	}

	if len(tlsConfig.Certificates) == 0 {
		t.Error("Expected client certificate to be loaded")
	}
}

func TestConfig_ClientConfig_WithCA(t *testing.T) {
	_, _, caFile := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(caFile))

	config := DefaultConfig()
	config.ClientCAFile = caFile

	tlsConfig, err := config.ClientConfig()
	if err != nil {
		t.Fatalf("Failed to create client config: %v", err)
	}

	if tlsConfig.RootCAs == nil {
		t.Error("Expected RootCAs to be set")
	}
}

func TestConfig_ClientConfig_InsecureSkipVerify(t *testing.T) {
	config := DefaultConfig()
	config.InsecureSkipVerify = true

	tlsConfig, err := config.ClientConfig()
	if err != nil {
		t.Fatalf("Failed to create client config: %v", err)
	}

	if !tlsConfig.InsecureSkipVerify {
		t.Error("Expected InsecureSkipVerify to be true")
	}
}

func TestMTLSConfig(t *testing.T) {
	certFile, keyFile, caFile := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := MTLSConfig(certFile, keyFile, caFile)

	if config.CertFile != certFile {
		t.Error("CertFile not set correctly")
	}
	if config.KeyFile != keyFile {
		t.Error("KeyFile not set correctly")
	}
	if config.CAFile != caFile {
		t.Error("CAFile not set correctly")
	}
	if config.ClientCertFile != certFile {
		t.Error("ClientCertFile not set correctly")
	}
	if !config.RequireClientCert {
		t.Error("Expected RequireClientCert to be true")
	}
}

func TestServerOnlyConfig(t *testing.T) {
	certFile, keyFile, _ := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := ServerOnlyConfig(certFile, keyFile)

	if config.CertFile != certFile {
		t.Error("CertFile not set correctly")
	}
	if config.KeyFile != keyFile {
		t.Error("KeyFile not set correctly")
	}
	if config.RequireClientCert {
		t.Error("Expected RequireClientCert to be false")
	}
}

func TestOptionalClientTLSConfig(t *testing.T) {
	certFile, keyFile, caFile := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := OptionalClientTLSConfig(certFile, keyFile, caFile)

	if config.CertFile != certFile {
		t.Error("CertFile not set correctly")
	}
	if !config.OptionalClientTLS {
		t.Error("Expected OptionalClientTLS to be true")
	}
	if config.RequireClientCert {
		t.Error("Expected RequireClientCert to be false")
	}
}

func TestLoadServerConfig(t *testing.T) {
	certFile, keyFile, caFile := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	tlsConfig, err := LoadServerConfig(certFile, keyFile, caFile, true)
	if err != nil {
		t.Fatalf("Failed to load server config: %v", err)
	}

	if tlsConfig == nil {
		t.Fatal("Expected non-nil TLS config")
	}
	if len(tlsConfig.Certificates) == 0 {
		t.Error("Expected certificates to be loaded")
	}
}

func TestLoadClientConfig(t *testing.T) {
	certFile, keyFile, caFile := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	tlsConfig, err := LoadClientConfig(certFile, keyFile, caFile)
	if err != nil {
		t.Fatalf("Failed to load client config: %v", err)
	}

	if tlsConfig == nil {
		t.Fatal("Expected non-nil TLS config")
	}
	if len(tlsConfig.Certificates) == 0 {
		t.Error("Expected client certificates to be loaded")
	}
}

func TestLoadMTLSConfig(t *testing.T) {
	certFile, keyFile, caFile := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	serverConfig, clientConfig, err := LoadMTLSConfig(certFile, keyFile, caFile)
	if err != nil {
		t.Fatalf("Failed to load mTLS config: %v", err)
	}

	if serverConfig == nil {
		t.Fatal("Expected non-nil server config")
	}
	if clientConfig == nil {
		t.Fatal("Expected non-nil client config")
	}

	// Verify server config requires client certs
	if serverConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Errorf("Expected server to require client certs, got %d", serverConfig.ClientAuth)
	}

	// Verify client config has certificates
	if len(clientConfig.Certificates) == 0 {
		t.Error("Expected client config to have certificates")
	}
}

func TestConfig_ServerConfig_InvalidCertFile(t *testing.T) {
	config := DefaultConfig()
	config.CertFile = "/nonexistent/cert.pem"
	config.KeyFile = "/nonexistent/key.pem"

	_, err := config.ServerConfig()
	if err == nil {
		t.Fatal("Expected error for invalid cert file")
	}
}

func TestConfig_ServerConfig_InvalidCAFile(t *testing.T) {
	certFile, keyFile, _ := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := DefaultConfig()
	config.CertFile = certFile
	config.KeyFile = keyFile
	config.CAFile = "/nonexistent/ca.pem"

	_, err := config.ServerConfig()
	if err == nil {
		t.Fatal("Expected error for invalid CA file")
	}
}

func TestConfig_ClientConfig_InvalidCertFile(t *testing.T) {
	config := DefaultConfig()
	config.ClientCertFile = "/nonexistent/cert.pem"
	config.ClientKeyFile = "/nonexistent/key.pem"

	_, err := config.ClientConfig()
	if err == nil {
		t.Fatal("Expected error for invalid client cert file")
	}
}

func TestConfig_ServerConfig_CipherSuites(t *testing.T) {
	certFile, keyFile, _ := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := DefaultConfig()
	config.CertFile = certFile
	config.KeyFile = keyFile
	config.CipherSuites = []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384}

	tlsConfig, err := config.ServerConfig()
	if err != nil {
		t.Fatalf("Failed to create server config: %v", err)
	}

	if len(tlsConfig.CipherSuites) != 1 {
		t.Errorf("Expected 1 cipher suite, got %d", len(tlsConfig.CipherSuites))
	}
	if tlsConfig.CipherSuites[0] != tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384 {
		t.Error("Cipher suite not set correctly")
	}
}

func TestConfig_ClientConfig_CipherSuites(t *testing.T) {
	config := DefaultConfig()
	config.CipherSuites = []uint16{tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384}

	tlsConfig, err := config.ClientConfig()
	if err != nil {
		t.Fatalf("Failed to create client config: %v", err)
	}

	if len(tlsConfig.CipherSuites) != 1 {
		t.Errorf("Expected 1 cipher suite, got %d", len(tlsConfig.CipherSuites))
	}
}

func TestConfig_ServerConfig_MinMaxVersion(t *testing.T) {
	certFile, keyFile, _ := setupTestCerts(t)
	defer cleanupTestCerts(t, filepath.Dir(certFile))

	config := DefaultConfig()
	config.CertFile = certFile
	config.KeyFile = keyFile
	config.MinVersion = tls.VersionTLS13
	config.MaxVersion = tls.VersionTLS13

	tlsConfig, err := config.ServerConfig()
	if err != nil {
		t.Fatalf("Failed to create server config: %v", err)
	}

	if tlsConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("Expected MinVersion TLS 1.3, got %d", tlsConfig.MinVersion)
	}
	if tlsConfig.MaxVersion != tls.VersionTLS13 {
		t.Errorf("Expected MaxVersion TLS 1.3, got %d", tlsConfig.MaxVersion)
	}
}

func TestConfig_ClientConfig_MinMaxVersion(t *testing.T) {
	config := DefaultConfig()
	config.MinVersion = tls.VersionTLS13
	config.MaxVersion = tls.VersionTLS13

	tlsConfig, err := config.ClientConfig()
	if err != nil {
		t.Fatalf("Failed to create client config: %v", err)
	}

	if tlsConfig.MinVersion != tls.VersionTLS13 {
		t.Errorf("Expected MinVersion TLS 1.3, got %d", tlsConfig.MinVersion)
	}
	if tlsConfig.MaxVersion != tls.VersionTLS13 {
		t.Errorf("Expected MaxVersion TLS 1.3, got %d", tlsConfig.MaxVersion)
	}
}

