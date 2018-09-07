package fridaynotice

import (
	"math"
	"os"
	"strings"
	"testing"
)

func NewTestRandomMessenger() (*RandomMessenger, error) {
	return NewRandomMessenger(RandomMessengerConfig{
		DSN:       os.Getenv("FRIDAYNOTICE_DSN"),
		Messages:  []string{"Test message"},
		SharedKey: "",
		Nonce:     "",
		ReturnUrl: "",
	})
}

func TestGetVolunteers(t *testing.T) {
	m, _ := NewTestRandomMessenger()
	users := m.GetVolunteers("")

	if len(users) == 0 {
		t.Log("At least 1 user should exists")
		t.Fail()
	}

	// loop to test username must be prefix with podd*
	for _, user := range users {
		if !strings.HasPrefix(user.Username, "podd") {
			t.Log("User results contain non-volunteers users")
			t.Fail()
			break
		}

		if user.Device.Type == DEVICE_TYPE_IOS {
			t.Logf("User devices now support only Android, user: %s", user.Username)
			t.Fail()
			break
		}

		if user.Device.RegId == "" {
			t.Log("User results should has device defined")
			t.Fail()
			break
		}
	}
}

func TestGetMessage(t *testing.T) {
	m, _ := NewTestRandomMessenger()
	message1 := m.GetMessage()

	if message1 == "" {
		t.Log("No message content")
		t.Fail()
	}
}

func TestGetRegIdsChunks(t *testing.T) {
	m, _ := NewTestRandomMessenger()
	users := m.GetVolunteers("")
	chunks := m.MakeRegIdsChunks(users, 10)

	if len(chunks) != int(math.Ceil(float64(len(users))/10.0)) {
		t.Logf("RegIds chunk size is not valid: %d instead of %d", len(chunks), len(users)/10)
		t.Fail()
	}

	if len(chunks[0]) != 10 {
		t.Logf("RegIds size is not valid: %d instead of %d", len(chunks[0]), 10)
		t.Fail()
	}
}

func TestSendNotification(t *testing.T) {
	m, _ := NewTestRandomMessenger()

	users := m.GetVolunteers("")
	chunks := m.MakeRegIdsChunks(users, 10)
	sender := &TestSender{ApiKey: "TEST_API_KEY"}
	m.SendNotification(sender, chunks)

	if sender.ReqCount != len(chunks) {
		t.Logf("gcm.Send() function is called %d times instead of %d", sender.ReqCount, len(chunks))
		t.Fail()
	}
}
