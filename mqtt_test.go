package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/caarlos0/env/v10"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/eclipse/paho.mqtt.golang/packets"
)

func TestMQTTRejectsUntrustedTLSBroker(t *testing.T) {
	broker := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("MQTT connection reached HTTP handler")
	}))
	defer broker.Close()
	host, port, err := net.SplitHostPort(broker.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	useTLS := true
	client := mqtt.NewClient(mqttClientOptions(Config{BrokerHost: host, BrokerPort: portNumber, BrokerTLS: &useTLS}))
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("TLS connection did not finish")
	}
	if token.Error() == nil || !strings.Contains(token.Error().Error(), "certificate") {
		t.Fatalf("expected certificate validation failure, got %v", token.Error())
	}
	if client.IsConnected() {
		client.Disconnect(0)
		t.Fatal("connected to an untrusted TLS broker")
	}
}

func TestMQTTTLSBrokerCredentials(t *testing.T) {
	certificateServer := httptest.NewTLSServer(http.NotFoundHandler())
	certificate := certificateServer.TLS.Certificates[0]
	trustedCertificate := certificateServer.Certificate()
	certificateServer.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	listener = tls.NewListener(listener, &tls.Config{Certificates: []tls.Certificate{certificate}})
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close synthetic broker listener: %v", err)
		}
	})
	result := make(chan error, 1)
	go func() {
		var resultErr error
		defer func() { result <- resultErr }()
		conn, err := listener.Accept()
		if err != nil {
			resultErr = err
			return
		}
		defer func() { resultErr = errors.Join(resultErr, conn.Close()) }()
		packet, err := packets.ReadPacket(conn)
		if err != nil {
			resultErr = err
			return
		}
		connect, ok := packet.(*packets.ConnectPacket)
		if !ok {
			resultErr = fmt.Errorf("expected CONNECT packet, got %T", packet)
			return
		}
		if connect.Username != "synthetic-user" || string(connect.Password) != "synthetic-password" {
			resultErr = fmt.Errorf("CONNECT credentials were not transmitted")
			return
		}
		resultErr = packets.NewControlPacket(packets.Connack).Write(conn)
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	useTLS := true
	options := mqttClientOptions(Config{
		BrokerHost:     host,
		BrokerPort:     portNumber,
		BrokerTLS:      &useTLS,
		BrokerUsername: "synthetic-user",
		BrokerPassword: "synthetic-password",
	})
	options.TLSConfig.RootCAs = x509.NewCertPool()
	options.TLSConfig.RootCAs.AddCert(trustedCertificate)
	options.SetAutoReconnect(false)
	client := mqtt.NewClient(options)
	defer client.Disconnect(0)
	token := client.Connect()
	if !token.WaitTimeout(5 * time.Second) {
		t.Fatal("MQTT connection did not finish")
	}

	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if err := token.Error(); err != nil {
		t.Fatalf("MQTT connection failed: %v", err)
	}
}

func TestMQTTTransportDefaults(t *testing.T) {
	for _, test := range []struct {
		name        string
		environment map[string]string
		scheme      string
	}{
		{"remote defaults to TLS", map[string]string{"MQTT_HOST": "broker.example"}, "ssl"},
		{"local defaults to plaintext", map[string]string{}, "tcp"},
		{"loopback defaults to plaintext", map[string]string{"MQTT_HOST": "127.0.0.2"}, "tcp"},
		{"IPv6 loopback defaults to plaintext", map[string]string{"MQTT_HOST": "::1"}, "tcp"},
		{"explicit local TLS", map[string]string{"MQTT_TLS": "true"}, "ssl"},
		{"explicit trusted remote plaintext", map[string]string{"MQTT_HOST": "broker.example", "MQTT_TLS": "false"}, "tcp"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var cfg Config
			test.environment["HOMEKIT_PIN"] = "03145154"
			if err := env.ParseWithOptions(&cfg, env.Options{Environment: test.environment}); err != nil {
				t.Fatal(err)
			}
			options := mqttClientOptions(cfg)
			if options.Servers[0].Scheme != test.scheme {
				t.Fatalf("expected %s transport, got %s", test.scheme, options.Servers[0].Scheme)
			}
			if test.scheme == "ssl" && (options.TLSConfig == nil || options.TLSConfig.ServerName != cfg.BrokerHost || options.TLSConfig.InsecureSkipVerify) {
				t.Fatal("TLS must verify the configured broker identity")
			}
		})
	}
}
