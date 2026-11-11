package main

import (
	"bytes"
	"crypto/sha512"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/brutella/hap"
	"github.com/brutella/hap/accessory"
	"github.com/brutella/hap/tlv8"
	"github.com/tadglines/go-pkgs/crypto/srp"
)

type pairingMessage struct {
	Method        byte   `tlv8:"0,optional"`
	Salt          []byte `tlv8:"2,optional"`
	PublicKey     []byte `tlv8:"3,optional"`
	Proof         []byte `tlv8:"4,optional"`
	EncryptedData []byte `tlv8:"5,optional"`
	State         byte   `tlv8:"6,optional"`
	Error         byte   `tlv8:"7,optional"`
}

func TestPairingRequiresPINProof(t *testing.T) {
	t.Run("missing proof", func(t *testing.T) {
		server, exchange := pairingServer(t)
		exchange(pairingMessage{State: hap.M1})
		response := exchange(pairingMessage{State: hap.M5, EncryptedData: make([]byte, 16)})
		if response.State != hap.M6 || response.Error != hap.TlvErrorInvalidRequest {
			t.Fatalf("expected M6 invalid request, got state=%d error=%d", response.State, response.Error)
		}
		if server.IsPaired() {
			t.Fatal("pairing was persisted without a verified PIN proof")
		}
	})

	t.Run("invalid proof", func(t *testing.T) {
		server, exchange := pairingServer(t)
		response := exchange(pairingMessage{State: hap.M1})
		client := pairingClient(t, response)
		response = exchange(pairingMessage{State: hap.M3, PublicKey: client.GetA(), Proof: []byte("invalid")})
		if response.Error != hap.TlvErrorInvalidRequest {
			t.Fatalf("expected rejected PIN proof, got error=%d", response.Error)
		}
		response = exchange(pairingMessage{State: hap.M5, EncryptedData: make([]byte, 16)})
		if response.State != hap.M6 || response.Error != hap.TlvErrorInvalidRequest {
			t.Fatalf("expected M6 invalid request after failed proof, got state=%d error=%d", response.State, response.Error)
		}
		if server.IsPaired() {
			t.Fatal("pairing was persisted after a failed PIN proof")
		}
	})

	t.Run("valid proof", func(t *testing.T) {
		_, exchange := pairingServer(t)
		response := exchange(pairingMessage{State: hap.M1})
		client := pairingClient(t, response)
		response = exchange(pairingMessage{State: hap.M3, PublicKey: client.GetA(), Proof: client.ComputeAuthenticator()})
		if response.State != hap.M4 || response.Error != 0 || !client.VerifyServerAuthenticator(response.Proof) {
			t.Fatalf("valid PIN proof rejected: state=%d error=%d", response.State, response.Error)
		}
		response = exchange(pairingMessage{State: hap.M5, EncryptedData: make([]byte, 16)})
		if response.State != hap.M6 || response.Error != hap.TlvErrorUnknown {
			t.Fatalf("expected ciphertext rejection after verified proof, got state=%d error=%d", response.State, response.Error)
		}
		response = exchange(pairingMessage{State: hap.M5, EncryptedData: make([]byte, 16)})
		if response.Error != hap.TlvErrorInvalidRequest {
			t.Fatalf("verified proof was reusable after M5: error=%d", response.Error)
		}
	})
}

func pairingServer(t *testing.T) (*hap.Server, func(pairingMessage) pairingMessage) {
	t.Helper()
	bridge := accessory.NewBridge(accessory.Info{Name: "Synthetic bridge"})
	server, err := hap.NewServer(hap.NewFsStore(t.TempDir()), bridge.A)
	if err != nil {
		t.Fatal(err)
	}
	server.Pin = "03145154"
	handler, ok := server.ServeMux().(http.Handler)
	if !ok {
		t.Fatal("pairing router does not implement http.Handler")
	}
	return server, func(message pairingMessage) pairingMessage {
		t.Helper()
		body, err := tlv8.Marshal(struct {
			Method        byte   `tlv8:"0"`
			PublicKey     []byte `tlv8:"3"`
			Proof         []byte `tlv8:"4"`
			EncryptedData []byte `tlv8:"5"`
			State         byte   `tlv8:"6"`
		}{message.Method, message.PublicKey, message.Proof, message.EncryptedData, message.State})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPost, "/pair-setup", bytes.NewReader(body))
		request.RemoteAddr = "192.0.2.1:1234"
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		var response pairingMessage
		if err := tlv8.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode pairing response: %v", err)
		}
		return response
	}
}

func pairingClient(t *testing.T, response pairingMessage) *srp.ClientSession {
	t.Helper()
	protocol, err := srp.NewSRP("rfc5054.3072", sha512.New, func(salt, password []byte) []byte {
		identity := sha512.Sum512(append([]byte("Pair-Setup:"), password...))
		hash := sha512.New()
		hash.Write(salt)
		hash.Write(identity[:])
		return hash.Sum(nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	client := protocol.NewClientSession([]byte("Pair-Setup"), []byte("031-45-154"))
	if _, err := client.ComputeKey(response.Salt, response.PublicKey); err != nil {
		t.Fatal(err)
	}
	return client
}
