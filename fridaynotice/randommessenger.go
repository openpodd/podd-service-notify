package fridaynotice

import (
	"database/sql"
	"fmt"
	"github.com/alexjlockwood/gcm"
	_ "github.com/lib/pq"
	"github.com/openpodd/podd-service-notify"
	"log"
	"math/rand"
	"strconv"
	"time"
)

type DeviceType int

const (
	DEVICE_TYPE_ANDROID DeviceType = iota
	DEVICE_TYPE_IOS
)

type Device struct {
	Type  DeviceType
	RegId string
}

type User struct {
	Username string
	Token    string
	Device   Device
}

type GCMMessage map[string]interface{}

const buttonTemplates = `
กดลิ้งค์เพื่อรายงาน <p><button id="submit-link" onclick="submit(); return false;" style="border:none;padding: 10px;color: #fff;margin: 15px 0 0;font-size: 18px;background-color: #1C95EF;"style="border:none;padding: 10px;color: #fff;margin: 15px 0 0;font-size: 18px;background-color: #1C95EF;">ไม่พบเหตุผิดปกติ</button></p>
<script>
var submitLink = document.getElementById('submit-link');

function submit() {
	var oReq = new XMLHttpRequest();

	oReq.onreadystatechange = function () {
		if (oReq.readyState === 4 && oReq.status === 200) {
			var wrapper = document.getElementsByClassName("wrapper");
			wrapper[0].innerText = "ขอบคุณสำหรับการรายงานค่ะ";
		}
	};

	oReq.open("GET", "%s");
	oReq.send();
}
</script>
`

type RandomMessengerConfig struct {
	DSN      string
	Messages []string

	SharedKey string
	Nonce     string
	ReturnUrl string

	ReportButtonEnabled bool
}

type RandomMessenger struct {
	DB     *sql.DB
	Config RandomMessengerConfig
	Cipher podd_service_notify.Cipher
}

func (m *RandomMessenger) GetVolunteers(username string) []*User {
	users := make([]*User, 0)

	queryString := `
		SELECT username, gcm_reg_id, t.key
		FROM accounts_user u
	    	join accounts_userdevice d on u.id = d.user_id
	    	join authtoken_token t on u.id = t.user_id
		WHERE gcm_reg_id != '' AND u.domain_id = 1
	`
	if username != "" {
		queryString += fmt.Sprintf(" AND username = '%s' ", username)
	} else {
		queryString += " AND username LIKE 'podd%' "
	}

	rows, err := m.DB.Query(queryString)
	if err != nil {
		log.Printf("Error fetching volunteers %v", err)
		return users
	}
	defer rows.Close()

	for rows.Next() {
		var username string
		var gcmRegId string
		var token string
		err = rows.Scan(&username, &gcmRegId, &token)
		if err != nil {
			panic(err)
		}

		users = append(users, &User{
			Username: username,
			Token:    token,
			Device: Device{
				Type:  DEVICE_TYPE_ANDROID,
				RegId: gcmRegId,
			},
		})
	}

	return users
}

func (m *RandomMessenger) GetMessage() string {
	rand.Seed(time.Now().Unix())
	return m.Config.Messages[rand.Intn(len(m.Config.Messages))]
}

func (m *RandomMessenger) MakeRegIdsChunks(users []*User, chunkSize int) [][]string {
	var chunks [][]string
	chunks = make([][]string, 0)

	userSize := len(users)
	// send chunk of `chunkSize` users
	for i := 0; i <= userSize/chunkSize; i++ {
		startIndex := chunkSize * i
		limit := (i + 1) * chunkSize
		if limit >= userSize {
			limit = userSize
		}

		var regIds []string
		regIds = make([]string, 0)
		for j := startIndex; j < limit; j++ {
			regIds = append(regIds, users[j].Device.RegId)
		}

		chunks = append(chunks, regIds)
	}

	return chunks
}

func (m *RandomMessenger) SendNotification(sender Sender, regIdsChunks [][]string) {
	messageId := strconv.Itoa(rand.Int())
	messageText := m.GetMessage()

	successCount := 0
	failCount := 0
	for _, regIds := range regIdsChunks {
		data := GCMMessage{
			"id":       messageId,
			"message":  messageText,
			"type":     "news",
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
	}

	log.Printf("Successfully sent GCM messages to %d devices, fail %d devices", successCount, failCount)
}

func (m *RandomMessenger) CreateGCMMessageTextForUser(user *User) string {
	cipher := m.Cipher

	messageText := m.GetMessage()

	payload, err := podd_service_notify.CreatePayload(user.Token, 0, time.Hour*24*7)
	if err == nil {
		payloadStr, err := cipher.EncodePayload(payload)
		if err != nil {
			log.Printf("Error coding payload for user %s", user.Username)
			log.Println(err)
		} else if m.Config.ReportButtonEnabled {
			messageText += fmt.Sprintf(buttonTemplates, m.Config.ReturnUrl+"/"+payloadStr)
		}
	}

	return messageText
}

func (m *RandomMessenger) SendNotificationToUser(sender Sender, user *User) {
	messageId := user.Username + "-" + strconv.Itoa(rand.Int())

	data := GCMMessage{
		"id":       messageId,
		"message":  m.CreateGCMMessageTextForUser(user),
		"type":     "news",
		"reportId": "",
	}
	regIds := []string{user.Device.RegId}
	message := gcm.NewMessage(data, regIds...)
	message.TimeToLive = 604800 // 60 * 60 * 24 * 7

	response, err := sender.Send(message, 3)
	if err != nil {
		log.Print("Fail with error", err, response)
	} else {
		log.Printf("Successfully sent GCM messages to username: %s\n", user.Username)
	}
}

func NewRandomMessenger(config RandomMessengerConfig) (*RandomMessenger, error) {
	db, err := sql.Open("postgres", config.DSN)
	if err != nil {
		return nil, err
	}
	m := RandomMessenger{
		DB:     db,
		Config: config,
		Cipher: podd_service_notify.Cipher{
			Key:   config.SharedKey,
			Nonce: config.Nonce,
		},
	}

	return &m, nil
}
