package podd_service_notify

import (
	"fmt"
	"net/http"
	"strings"
)

type RefNoCache interface {
	Exists(key string) bool
	Set(key string, value string) error
}

type Callback interface {
	Execute(payload Payload) bool
}

type Server struct {
	Cipher Cipher
	Cache  RefNoCache
}

// return true when refNo already processed
func ValidateRefNo(cache RefNoCache, refNo string) bool {
	if cache.Exists(refNo) {
		return true
	} else {
		cache.Set(refNo, "1")
		return false
	}
}

func (s Server) ZeroReportHandler(callback Callback) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		
		urlPart := strings.Split(r.URL.Path, "/")
		payload, err := s.Cipher.DecodePayload(urlPart[len(urlPart) - 1])
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// expire
		if payload.IsExpired() {
			fmt.Println("Payload is expired")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// refno
		if ValidateRefNo(s.Cache, payload.RefNo) {
			fmt.Println("Payload is already processed")
			w.WriteHeader(http.StatusOK)
			return
		} else {
			fmt.Println("Payload is a new one")
		}

		if callback != nil {
			success := callback.Execute(payload)
			if ! success {
				fmt.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ขอบคุณสำหรับการรายงานค่ะ"))
	}
}

func (s Server) VerifyReportHandler(callback Callback) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		urlPart := strings.Split(r.URL.Path, "/")
		payload, err := s.Cipher.DecodePayload(urlPart[len(urlPart) - 1])
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// expire
		if payload.IsExpired() {
			fmt.Println("Payload is expired")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// refno
		if ValidateRefNo(s.Cache, payload.RefNo) {
			fmt.Println("Payload is already processed")
			w.WriteHeader(http.StatusOK)
			return
		} else {
			fmt.Println("Payload is a new one")
		}

		if callback != nil {
			success := callback.Execute(payload)
			if ! success {
				fmt.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
	}
}
