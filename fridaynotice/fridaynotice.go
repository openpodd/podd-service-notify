package main

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"math/rand"
	"time"
	"github.com/alexjlockwood/gcm"
	"strconv"
	"fmt"
	"github.com/openpodd/podd-service-notify"
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

var messages = []string{
	"อาสาผ่อดีดีตรวจสอบเหตุการณ์ในพื้นที่ของตนเอง ถ้าไม่มีสิ่งใดผิดปกติ กรุณาส่งรายงานไม่พบเหตุการณ์ผิดปกติมายังโครงการผ่อดีดีด้วย ขอบคุณค่ะ	",
}

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

func GetDB() (*sql.DB, error) {
	return sql.Open("postgres", *dsn)
}

func GetVolunteers() []*User {
	users := make([]*User, 0)

	db, _ := GetDB()
	rows, err := db.Query(`
	SELECT username, gcm_reg_id, t.key
	FROM accounts_user u
	     join accounts_userdevice d on u.id = d.user_id
	     join authtoken_token t on u.id = t.user_id
	WHERE username LIKE 'podd%' AND gcm_reg_id != '' AND u.domain_id = 1`)
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
			Token: token,
			Device: Device{
				Type: DEVICE_TYPE_ANDROID,
				RegId: gcmRegId,
			},
		})
	}

	return users
}

func GetMessage() string {
	rand.Seed(time.Now().Unix())
	return messages[rand.Intn(len(messages))]
}

func MakeRegIdsChunks(users []*User, chunkSize int) [][]string {
	var chunks [][]string
	chunks = make([][]string, 0)

	userSize := len(users)
	// send chunk of `chunkSize` users
	for i := 0; i <= userSize / chunkSize; i++ {
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

func SendNotification(sender Sender, regIdsChunks [][]string) {
	messageId := strconv.Itoa(rand.Int())
	messageText := GetMessage()

	successCount := 0
	failCount := 0
	for _, regIds := range (regIdsChunks) {
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
	}

	log.Printf("Successfully sent GCM messages to %d devices, fail %d devices", successCount, failCount)
}

func CreateGCMMessageTextForUser(user *User) string {
	cipher := podd_service_notify.Cipher{
		Key: *sharedKey,
		Nonce: *nonce,
	}

	messageText := GetMessage()

	payload, err := podd_service_notify.CreatePayload(user.Token, 0, time.Hour * 24 * 7)
	if err == nil {
		payloadStr, err := cipher.EncodePayload(payload)
		if err != nil {
			log.Printf("Error coding payload for user %s", user.Username)
			log.Println(err)
		} else {
			messageText += fmt.Sprintf(buttonTemplates, *returnServerUrl + "/" + payloadStr)
		}
	}

	return messageText
}

func SendNotificationToUser(sender Sender, user *User) {
	messageId := user.Username + "-" + strconv.Itoa(rand.Int())

	data := GCMMessage{
		"id": messageId,
		"message": CreateGCMMessageTextForUser(user),
		"type": "news",
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