package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"time"
)

func main() {
	originIP := []net.IP{[]byte{127, 0, 0, 1}, []byte{152, 94, 1, 149}, []byte{152, 94, 1, 145}, []byte{152, 94, 1, 152}} //27,31,34

	//SERVER
	// generate a new key-pair
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("generating random key: %v", err)
	}

	serverCertTmpl, err := CertTemplate()
	if err != nil {
		log.Fatalf("creating cert template: %v", err)
	}
	// describe what the certificate will be used for
	serverCertTmpl.IsCA = true
	serverCertTmpl.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	serverCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	serverCertTmpl.IPAddresses = originIP

	serverCert, serverCertPEM, err := CreateCert(serverCertTmpl, serverCertTmpl, &serverKey.PublicKey, serverKey)
	if err != nil {
		log.Fatalf("error creating cert: %v", err)
	}

	//fmt.Printf("%#x\n", serverCert.Signature) // more ugly binary

	// PEM encode the private key
	serverKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(serverKey),
	})

	// Create a TLS cert using the private key and certificate
	serverTLSCert, err := tls.X509KeyPair(serverCertPEM, serverKeyPEM)
	if err != nil {
		log.Fatalf("invalid key pair: %v", err)
	}

	//CLIENT
	// create a key-pair for the clienter
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("generating random key: %v", err)
	}

	// create a template for the clienter
	clientCertTmpl, err := CertTemplate()
	if err != nil {
		log.Fatalf("creating cert template: %v", err)
	}
	clientCertTmpl.KeyUsage = x509.KeyUsageDigitalSignature
	clientCertTmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	clientCertTmpl.IPAddresses = originIP

	// create a certificate which wraps the clienter's public key, sign it with the server private key
	_, clientCertPEM, err := CreateCert(clientCertTmpl, serverCert, &clientKey.PublicKey, serverKey)
	if err != nil {
		log.Fatalf("error creating cert: %v", err)
	}

	// provide the private key and the cert
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey),
	})
	clientTLSCert, err := tls.X509KeyPair(clientCertPEM, clientKeyPEM)
	if err != nil {
		log.Fatalf("invalid key pair: %v", err)
	}

	//fmt.Printf("%s\n", serverCertPEM)
	//fmt.Printf("%s\n", serverKeyPEM)
	//fmt.Printf("%s\n", clientCertPEM)
	//fmt.Printf("%s\n", clientKeyPEM)
	err = ioutil.WriteFile("serverCert.crt", serverCertPEM, 0644)
	if err != nil {
		log.Fatalf("failed to write to file: %v", err)
	}
	err = ioutil.WriteFile("serverKey.key", serverKeyPEM, 0644)
	if err != nil {
		log.Fatalf("failed to write to file: %v", err)
	}
	err = ioutil.WriteFile("clientCert.crt", clientCertPEM, 0644)
	if err != nil {
		log.Fatalf("failed to write to file: %v", err)
	}
	err = ioutil.WriteFile("clientKey.key", clientKeyPEM, 0644)
	if err != nil {
		log.Fatalf("failed to write to file: %v", err)
	}

	//TESTPROCEDURE
	ok := func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("HI!")) }
	s := httptest.NewUnstartedServer(http.HandlerFunc(ok))

	// Configure the clienter to present the certficate we created
	s.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverTLSCert},
	}
	// create another test clienter and use the certificate
	s = httptest.NewUnstartedServer(http.HandlerFunc(ok))
	s.TLS = &tls.Config{
		Certificates: []tls.Certificate{clientTLSCert},
	}

	// create a pool of trusted certs
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(serverCertPEM)

	// configure a client to use trust those certificates
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: certPool},
		},
	}
	s.StartTLS()
	resp, err := client.Get(s.URL)
	s.Close()
	if err != nil {
		log.Fatalf("could not make GET request: %v", err)
	}
	dump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		log.Fatalf("could not dump response: %v", err)
	}
	fmt.Printf("%s\n", dump)
}

func CreateCert(template, parent *x509.Certificate, pub interface{}, parentPriv interface{}) (
	cert *x509.Certificate, certPEM []byte, err error) {

	certDER, err := x509.CreateCertificate(rand.Reader, template, parent, pub, parentPriv)
	if err != nil {
		return
	}
	// parse the resulting certificate so we can use it again
	cert, err = x509.ParseCertificate(certDER)
	if err != nil {
		return
	}
	// PEM encode the certificate (this is a standard TLS encoding)
	b := pem.Block{Type: "CERTIFICATE", Bytes: certDER}
	certPEM = pem.EncodeToMemory(&b)
	return
}

// helper function to create a cert template with a serial number and other required fields
func CertTemplate() (*x509.Certificate, error) {
	// generate a random serial number (a real cert authority would have some logic behind this)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.New("failed to generate serial number: " + err.Error())
	}

	tmpl := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"Yhat, Inc."}},
		SignatureAlgorithm:    x509.SHA256WithRSA,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour), // valid for an hour
		BasicConstraintsValid: true,
	}
	return &tmpl, nil
}
