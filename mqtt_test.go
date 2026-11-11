package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

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
	client := mqtt.NewClient(mqttClientOptions(Config{BrokerHost: host, BrokerPort: portNumber, BrokerTLS: true}))
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
	defer listener.Close()
	result := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			result <- err
			return
		}
		defer conn.Close()
		packet, err := packets.ReadPacket(conn)
		if err != nil {
			result <- err
			return
		}
		connect, ok := packet.(*packets.ConnectPacket)
		if !ok {
			result <- fmt.Errorf("expected CONNECT packet, got %T", packet)
			return
		}
		if connect.Username != "synthetic-user" || string(connect.Password) != "synthetic-password" {
			result <- fmt.Errorf("CONNECT credentials were not transmitted")
			return
		}
		result <- packets.NewControlPacket(packets.Connack).Write(conn)
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	options := mqttClientOptions(Config{
		BrokerHost:     host,
		BrokerPort:     portNumber,
		BrokerTLS:      true,
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
