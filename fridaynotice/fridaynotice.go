package main

import (
	"database/sql"
	_ "github.com/lib/pq"
	"log"
	"math/rand"
	"time"
	"github.com/alexjlockwood/gcm"
	"strconv"
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
	Device   Device
}

type GCMMessage map[string]interface{}

var messages = []string {
	"น้ำมาปลากินมด น้ำลดมดกินปลา",
	"ชักแม่น้ำทั้งห้า",
}

func GetDB() (*sql.DB, error) {
	return sql.Open("postgres", *dsn)
}

func GetVolunteers() []*User {
	users := make([]*User, 0)

	db, _ := GetDB()
	rows, err := db.Query("SELECT username, gcm_reg_id FROM accounts_user u join accounts_userdevice d on u.id = d.user_id WHERE username LIKE 'podd%' AND gcm_reg_id != ''")
	if err != nil {
		log.Printf("Error fetching volunteers %v", err)
		return users
	}
	defer rows.Close()

	for rows.Next() {
		var username string
		var gcmRegId string
		err = rows.Scan(&username, &gcmRegId)
		if err != nil {
			panic(err)
		}

		users = append(users, &User{
			Username: username,
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
	for _, regIds := range(regIdsChunks) {
		data := GCMMessage{
			"id": messageId,
			"message": messageText,
			"type": "news",
			"reportId": "",
		}
		message := gcm.NewMessage(data, regIds...)

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