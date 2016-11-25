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
)

type RedisCache struct {
	Client redis.Conn
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
จากการที่ท่านได้รายงาน โดยมีรายละเอียดดังนี้

%s

โปรดยืนยันว่าเป็นรายงานจริง
`

type ZeroReportCallback struct{}

func (c ZeroReportCallback) Execute(payload PoddService.Payload) bool {
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
	fmt.Println(resp)
	return resp.StatusCode == http.StatusCreated && err == nil
}

type VerifyReportCallback struct{}

func (c VerifyReportCallback) Execute(payload PoddService.Payload) bool {
	client := &http.Client{}

	url := fmt.Sprintf("%s/report/%d/protect-verify-case/%s/", *poddAPIURL, payload.Id, *poddSharedKey)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		fmt.Println(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Verify error", err)
		return false
	}
	return resp.StatusCode == http.StatusOK && err == nil
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

			if report.IsStateChanged &&
				report.ParentId == 0 &&
				report.ReportTypeId == *acceptedReportTypeId &&
				report.StateCode == *acceptedReportStateCode {

				// get gcm id
				var username string
				var gcmRegId string
				rows, err := db.Query(`
					SELECT u.username, gcm_reg_id
					FROM accounts_user u join accounts_userdevice d on u.id = d.user_id
					WHERE u.id = $1  AND gcm_reg_id != ''
				`, report.CreatedById)
				if err != nil {
					log.Println("Error querying gcm reg id", err)
					return
				}

				rows.Next()
				rows.Scan(&username, &gcmRegId)

				if gcmRegId != "" {
					log.Printf("  / -> Sending verify notification to user : %s (%d), device: %s\n", username, report.CreatedById, gcmRegId)

					gcmMessage := fmt.Sprintf(gcmTemplate, report.FormDataExplanation)
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

	// TODO: FIX THIS, Now error occured and make whole request stop.
	//db, err := sql.Open("postgres", *dbDSN)
	//if err != nil {
	//	panic(err)
	//}
	//sender := PoddService.NewSender(*gcmAPIKey)
	//
	//var wg sync.WaitGroup
	//wg.Add(1)
	//
	//go func() {
	//	defer wg.Done()
	//	doSubscribeReport(conn, db, sender)
	//}()

	http.HandleFunc("/report/zero/", server.ZeroReportHandler(ZeroReportCallback{}))
	http.HandleFunc("/report/verify/", server.VerifyReportHandler(VerifyReportCallback{}))
	http.ListenAndServe(":9800", nil)

	//wg.Wait()
}