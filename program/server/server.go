package main

import (
	"net/http"
	PoddService "github.com/openpodd/podd-service-notify"
	"github.com/vharitonsky/iniflags"
	"flag"
	"github.com/garyburd/redigo/redis"
	"fmt"
	"time"
	"strings"
	"encoding/json"
	"log"
	"database/sql"
	_ "github.com/lib/pq"
	"sync"
)

var (
	keyFlag = flag.String("key", "1234567890123456", "Key to encrypt/decrypt")
	nonceFlag = flag.String("nonce", "3a0117f29cd4261bab54b0f1", "Nonce")
	redisHostFlag = flag.String("redis.host", "127.0.0.1", "Redis host")
	redisPortFlag = flag.Int("redis.port", 6379, "Redis port")
	poddAPIURL = flag.String("api.url", "http://localhost:8000", "PODD API URL")
	poddSharedKey = flag.String("api.sharedKey", "must-override-in-settings-local.py", "PODD Shared Key")
	gcmAPIKey = flag.String("gcm.key", "local-sample-key", "GCM API Key")
	acceptedReportTypeId = flag.Int("report.typeId", 0, "Accepted Report Type Id")
	acceptedReportStateCode = flag.String("report.stateCode", "case", "Accepted Report State Code")
	dbDSN = flag.String("db.dsn", "user=postgres password=postgres dbname=postgres host=localhost port=5432 sslmode=disable", "Accepted Report State Code")
	verifyServerUrl = flag.String("verifyServerUrl", "http://localhost:9110/report/verify/", "Verify server url")
)

type RedisCache struct {
	Client redis.Conn
}

type DeviceType int

const (
	DEVICE_TYPE_ANDROID DeviceType = iota
	DEVICE_TYPE_IOS
)

type User struct {
	Username string
	Token    string
	Device   Device
}

type Device struct {
	Type  DeviceType
	RegId string
}

const zeroReport = `
{
  "incidentDate": "%s",
  "date": "%s",
  "reportTypeId": 0,
  "reportId": %d,
  "guid": "webcontent-%s",
  "negative": false
}
`

const gcmTemplate = `
<p>ตามที่อาสาได้รายงาน %s</p>
<p>
	<strong><u>กรุณากรอกข้อมูลเพื่อยืนยันรายงาน</u></strong>
	(เมื่อยืนยันแล้ว กรณีที่เป็นจริง ระบบจะทำการส่งข้อมูลแจ้งเตือนไปยัง ปศุสัตว์อำเภอ/จังหวัด และ องค์การปกครองส่วนท้องถิ่น)
</p>

<hr style= "border:none;border-top: 1px solid #ccc;"/>

<iframe src="%s" frameborder="0" scrolling="no" width="100%" height="600px">
</iframe>
`

const ThankyouTemplate = `
<style>
body {
    font-family: sans-serif;
    font-size: 20px;
    line-height: 1.5em;
    padding: 5px;
}
</style>
<p>ขอบคุณสำหรับการยืนยันรายงานค่ะ</p>
`

func createGCMMessageTextForUser(user *User, report *Report) string {
	cipher := PoddService.Cipher{
		Key: *keyFlag,
		Nonce: *nonceFlag,
	}

	var messageText string

	payload, err := PoddService.CreatePayload(user.Token, report.Id, time.Hour * 24 * 7)
	if err == nil {
		payloadStr, err := cipher.EncodePayload(payload)
		if err != nil {
			log.Printf("Error coding payload for user %s", user.Username)
			log.Println(err)
		} else {
			messageText = fmt.Sprintf(gcmTemplate, report.FormDataExplanation, *verifyServerUrl + payloadStr)
		}
	}

	return messageText
}

type ZeroReportCallback struct{}

func (c ZeroReportCallback) Execute(payload PoddService.Payload) (string, bool) {
	client := &http.Client{}

	date := time.Now().Local()
	zeroReportJSON := fmt.Sprintf(zeroReport, date.Format("2006-01-02"), date.Format(time.RFC3339), date.Unix(), payload.RefNo)

	url := fmt.Sprintf("%s/reports/", *poddAPIURL)
	req, err := http.NewRequest("POST", url, strings.NewReader(zeroReportJSON))
	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Token " + payload.Token)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Zero report error", err)
		return "", false
	}

	if resp.StatusCode == http.StatusCreated {
		return ThankyouTemplate, true
	} else {
		return "", false
	}
}

type VerifyReportCallback struct{}

func (c VerifyReportCallback) Execute(payload PoddService.Payload) (string, bool) {
	client := &http.Client{}

	verified := "no"
	println("Payload : isVerified", payload.Form.Get("isVerified"))
	println("Payload : isOutbreak", payload.Form.Get("isOutbreak"))

	if payload.Form.Get("isVerified") == "1" {
		verified = "yes"
	}

	extraInfo := "สถานการณ์ไม่ลุกลาม"
	if payload.Form.Get("isOutbreak") == "1" {
		extraInfo = "สถานการณ์ลุกลาม"
	}

	targetUrl := fmt.Sprintf("%s/report/%d/protect-verify-case/%s/%s/", *poddAPIURL, payload.Id, *poddSharedKey, verified)
	req, err := http.NewRequest("POST", targetUrl, nil)
	if err != nil {
		fmt.Println("Post to API error", err)
		return "", false
	}

	q := req.URL.Query()
	q.Add("extraInfo", extraInfo)
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Verify error", err)
		return "", false
	}

	if resp.StatusCode == http.StatusOK {
		return ThankyouTemplate, true
	} else {
		return "", false
	}
}

type FormData struct {
	AnimalType      string `json:"animalType"`
	AnimalTypeOther string `json:"animalTypeOther"`
}

type Report struct {
	Id                        int      `json:"id"`
	ParentId                  int      `json:"parent"`
	ReportTypeId              int      `json:"reportTypeId"`
	FormData                  FormData `json:"formData"`
	AdministrationAreaAddress string   `json:"administrationAreaAddress"`
	FormDataExplanation       string   `json:"formDataExplanation"`
	IsStateChanged            bool     `json:"isStateChanged"`
	StateCode                 string   `json:"stateCode"`
	CreatedById               int      `json:"createdById"`
	TestFlag                  bool     `json:"testFlag"`
}

func (r RedisCache) Exists(refNo string) bool {
	// check redis key
	value, err := r.Client.Do("EXISTS", refNo)
	if err != nil {
		panic(err)
	}

	return value == int64(1)
}

func (r RedisCache) Set(key string, value string) error {
	_, err := r.Client.Do("SET", key, value)
	return err
}

func doSubscribeReport(conn redis.Conn, db *sql.DB, sender PoddService.Sender) {
	psc := redis.PubSubConn{conn}
	psc.Subscribe("report:new")

	for {
		switch msg := psc.Receive().(type) {
		case redis.Message:
			log.Printf("%s: message: %s\n", msg.Channel, msg.Data)

			dec := json.NewDecoder(strings.NewReader(string(msg.Data)))

			var report Report
			err := dec.Decode(&report)
			if err != nil {
				log.Fatal(err)
			}

			log.Println("Got new report")
			log.Printf("  / reportId: %d, animalType: %s, stateCode: %s", report.Id, report.FormData.AnimalType, report.StateCode)

			if !report.TestFlag &&
				report.IsStateChanged &&
				report.ParentId == 0 &&
				report.ReportTypeId == *acceptedReportTypeId &&
				report.StateCode == *acceptedReportStateCode {

				// get gcm id
				var username string
				var gcmRegId string
				var token string
				rows, err := db.Query(`
					SELECT u.username, gcm_reg_id, t.key
					FROM accounts_user u
						 JOIN accounts_userdevice d on u.id = d.user_id
						 JOIN authtoken_token t on u.id = t.user_id
					WHERE u.id = $1  AND gcm_reg_id != ''
				`, report.CreatedById)
				if err != nil {
					log.Println("Error querying gcm reg id", err)
					return
				}

				rows.Next()
				rows.Scan(&username, &gcmRegId, &token)
				user := User{
					Username: username,
					Token: token,
					Device: Device{
						Type: DEVICE_TYPE_ANDROID,
						RegId: gcmRegId,
					},
				}

				if gcmRegId != "" {
					log.Printf("  / -> Sending verify notification to user : %s (%d), device: %s\n", username, report.CreatedById, gcmRegId)

					gcmMessage := createGCMMessageTextForUser(&user, &report)
					PoddService.SendNotification(sender, gcmRegId, gcmMessage)
				}
			} else {
				log.Println("  / -> gonna ignore it")
			}
		case redis.Subscription:
			log.Printf("%s: %s %d\n", msg.Channel, msg.Kind, msg.Count)
		case error:
			log.Println("Got new message and then error", msg.Error())
		}
	}
}

func main() {
	iniflags.Parse()

	conn, err := redis.Dial("tcp", fmt.Sprintf("%s:%d", *redisHostFlag, *redisPortFlag))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	redisCache := RedisCache{
		Client: conn,
	}

	server := PoddService.Server{
		Cipher: PoddService.Cipher{
			Key: *keyFlag,
			Nonce: *nonceFlag,
		},
		Cache: redisCache,
	}

	db, err := sql.Open("postgres", *dbDSN)
	if err != nil {
		panic(err)
	}
	sender := PoddService.NewSender(*gcmAPIKey)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		conn, err := redis.Dial("tcp", fmt.Sprintf("%s:%d", *redisHostFlag, *redisPortFlag))
		if err != nil {
			panic(err)
		}
		defer conn.Close()
		doSubscribeReport(conn, db, sender)
	}()

	http.HandleFunc("/report/zero/", server.ZeroReportHandler(ZeroReportCallback{}))
	http.HandleFunc("/report/verify/", server.VerifyReportHandler(VerifyReportCallback{}))
	http.ListenAndServe(":9800", nil)

	//wg.Wait()
}