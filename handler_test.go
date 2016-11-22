package podd_service_notify

import (
	"testing"
	"net/http"
	"net/http/httptest"
	"time"
	"fmt"
)

type MemoryCache struct {
	Map map[string]string
}

func (m MemoryCache) Exists(refNo string) bool {
	if _, ok := m.Map[refNo]; ok {
		return true
	} else {
		return false
	}
}

func (m MemoryCache) Set(key string, value string) error {
	m.Map[key] = value
	return nil
}

func TestZeroReportHandlerExpired(t *testing.T) {
	req, err := http.NewRequest("GET", "/report/zero/f8b0c1afb84f65264835f26738e76b8abfb6b32bbb17bebb6996c999204ba8f469321c39ae9e772d98c29a8cf43762374c177704bf0f04932925f3b473be3cc8d22a395a3f024b4eafcedf0643ef3f9d2cf7c4c3021cdec76eff303683ff79e07b5ec04c898818bcdeff0fda5d0256805613126d1433076f885770", nil)
	if err != nil {
		t.Fatal(err)
	}

	server := Server{
		Cipher: Cipher{
			Key: "1234567890123456",
			Nonce: "3a0117f29cd4261bab54b0f1",
		},
		Cache: MemoryCache{
			Map: make(map[string]string),
		},
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.ZeroReportHandler(nil))

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestZeroReportHandler(t *testing.T) {
	server := Server{
		Cipher: Cipher{
			Key: "1234567890123456",
			Nonce: "3a0117f29cd4261bab54b0f1",
		},
		Cache: MemoryCache{
			Map: make(map[string]string),
		},
	}

	payload, _ := CreatePayload("1234", 1234, time.Second * 1000)
	payloadStr, _ := server.Cipher.EncodePayload(payload)
	fmt.Println(payloadStr)

	req, err := http.NewRequest("GET", "/report/zero/" + payloadStr, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.ZeroReportHandler(nil))

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	handler.ServeHTTP(rr, req)
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
}

