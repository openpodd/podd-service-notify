package podd_service_notify

import (
	"github.com/alexjlockwood/gcm"
	"net/http"
	"strconv"
	"log"
	"math/rand"
)

type Sender interface {
	Send(*gcm.Message, int) (*gcm.Response, error)
}

type GCMMessage map[string]interface{}

type RealSender struct {
	ApiKey string
	Http *http.Client
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
	ApiKey string
	Http *http.Client
	ReqCount int
}

func (s *TestSender) Send(msg *gcm.Message, retries int) (*gcm.Response, error) {
	s.ReqCount++

	return &gcm.Response{
		Success: len(msg.RegistrationIDs),
		Failure: 0,
	}, nil
}

func SendNotification(sender Sender, regId string, messageText string) {
	messageId := strconv.Itoa(rand.Int())

	successCount := 0
	failCount := 0

	regIds := make([]string, 1)
	regIds[0] = regId

	data := GCMMessage{
		"id": messageId,
		"message": messageText,
		"type": "news",
		"reportId": "",
	}
	message := gcm.NewMessage(data, regIds...)
	message.TimeToLive = 604800 // 60 * 60 * 24 * 7

	response, err := sender.Send(message, 3)
	if err != nil {
		log.Print("Fail with error ", err, response)

		if response != nil {
			failCount += response.Failure
		} else {
			failCount += len(regIds)
		}
	} else {
		successCount += response.Success
	}

	log.Printf("Successfully sent GCM messages to %d devices, fail %d devices", successCount, failCount)
}