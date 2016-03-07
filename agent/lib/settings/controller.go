package settings

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"
)

/*
ControllerClient represents an active agent controller connection.
*/
type ControllerClient struct {
	URL    string
	Client *http.Client
	Config *Controller
}

func getHTTPClient(security *Security) *http.Client {
	var tlsConfig tls.Config

	if security.CertificateAuthority != "" {
		pem, err := ioutil.ReadFile(security.CertificateAuthority)
		if err != nil {
			log.Fatalf("%s", err)
		}

		tlsConfig.RootCAs = x509.NewCertPool()
		tlsConfig.RootCAs.AppendCertsFromPEM(pem)
	}

	if security.ClientCertificate != "" {
		if security.ClientCertificateKey == "" {
			log.Fatalf("Missing certificate key file")
		}

		cert, err := tls.LoadX509KeyPair(security.ClientCertificate,
			security.ClientCertificateKey)
		if err != nil {
			log.Fatalf("%s", err)
		}

		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return net.DialTimeout(network, addr, 10*time.Second)
			},
			DisableKeepAlives:   true,
			Proxy:               http.ProxyFromEnvironment,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     &tlsConfig,
		},
	}
}

/*
NewControllerClient gets a new agent controller connection
*/
func (c *Controller) GetClient() *ControllerClient {
	client := &ControllerClient{
		URL:    strings.TrimRight(c.URL, "/"),
		Client: getHTTPClient(&c.Security),
		Config: c,
	}

	return client
}

/*
BuildURL builds the request URL for agent
*/
func (client *ControllerClient) BuildURL(endpoint string) string {
	return fmt.Sprintf("%s/%d/%d/%s", client.URL,
		Options.Gid(),
		Options.Nid(),
		endpoint)
}
