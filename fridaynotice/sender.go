package fridaynotice

import (
	"github.com/alexjlockwood/gcm"
	"net/http"
)

type Sender interface {
	Send(*gcm.Message, int) (*gcm.Response, error)
}

type RealSender struct {
	ApiKey    string
	Http      *http.Client
	GCMSender *gcm.Sender
}

func (s *RealSender) Send(msg *gcm.Message, retries int) (*gcm.Response, error) {
	return s.GCMSender.Send(msg, retries)
}

func NewSender(apiKey string) *RealSender {
	gcmSender := gcm.Sender{ApiKey: apiKey, Http: http.DefaultClient}
	return &RealSender{GCMSender: &gcmSender}
}

type TestSender struct {
	ApiKey   string
	Http     *http.Client
	ReqCount int
}

func (s *TestSender) Send(msg *gcm.Message, retries int) (*gcm.Response, error) {
	s.ReqCount++

	return &gcm.Response{
		Success: len(msg.RegistrationIDs),
		Failure: 0,
	}, nil
}
