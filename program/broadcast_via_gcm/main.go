package main

import (
	"sync"
	"log"
	"encoding/json"
	"strings"
	"github.com/spf13/viper"
	"gopkg.in/redis.v3"
	"text/template"
	"bytes"

	"database/sql"
	_ "github.com/lib/pq"
)

type Report struct {
	Id                        int      `json:"id"`
	ParentId                  int      `json:"parent"`
	ReportTypeId              int      `json:"reportTypeId"`
	ReportTypeName            string   `json:"reportTypeName"`
	FormData                  FormData `json:"formData"`
	AdministrationAreaAddress string   `json:"administrationAreaAddress"`
	FormDataExplanation       string   `json:"formDataExplanation"`
	StateCode                 string   `json:"stateCode"`
	Date                      string   `json:"date"`
	CreatedById               int      `json:"createdById"`
	CreatedByName             string   `json:"createdByName"`
	CreatedByContact          string   `json:"createdByContact"`
	IsPublic                  bool     `json:"isPublic"`
	IsStateChanged            bool     `json:"isStateChanged"`
}

type FormData struct {
	AnimalType      string `json:"animalType"`
	AnimalTypeOther string `json:"animalTypeOther"`
	Symptom         string `json:"symptom"`
}

type RedisMessage struct {
	GcmApiKey     string     `json:"GCMAPIKey"`
	AndroidRegIds []string   `json:"androidRegistrationIds"`
	ApnsRegIds    []string   `json:"apnsRegistrationIds"`
	Type          string     `json:"type"`
	Message       string     `json:"message"`
	ReportId      int64      `json:"reportId"`
}

func haltOnErr(err error) {
	if err != nil {
		panic(err)
	}
}

var tmpl *template.Template
var db *sql.DB

func init() {
	var err error
	tmpl, err = template.New("message").Parse("" +
		"มีรายงานประเภท {{.ReportTypeName}} {{.FormDataExplanation}} \n" +
		"ที่ {{.AdministrationAreaAddress}} \n" +
		"รายงานโดย {{.CreatedByName}} \n" +
		"{{if .CreatedByContact}}ติดต่อกลับ {{.CreatedByContact}}{{end}}")
	if err != nil {
		panic(err)
	}

	viper.SetDefault("PODD_CALLBACK_URL", "")
	viper.SetDefault("PODD_API_TOKEN", "")

	viper.SetDefault("RedisAddr", "127.0.0.1:6379")
	viper.SetDefault("RedisDB", 0)

	viper.SetEnvPrefix("podd")
	viper.AutomaticEnv()

	viper.SetConfigType("json")
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		log.Printf("No config file: %s \n", err)
	}

	db, err = sql.Open("postgres", viper.GetString("PODD_POSTGRES"))
	if err != nil {
		panic(err)
	}
}

func main() {
	redisOptions := redis.Options{
		Addr: viper.GetString("RedisAddr"),
		DB:   int64(viper.GetInt("RedisDB")),
	}
	client := redis.NewClient(&redisOptions)
	pubsub, err := client.Subscribe("report:new")
	haltOnErr(err)

	log.Println("Waiting...")

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		for {
			msg, recErr := pubsub.ReceiveMessage()
			if recErr != nil {
				log.Printf("Receive error: %s", recErr)
			}

			dec := json.NewDecoder(strings.NewReader(msg.Payload))

			var report Report
			err := dec.Decode(&report)
			if err != nil {
				log.Fatal(err)
			}

			if report.IsStateChanged &&
				report.ParentId == 0 &&
				report.ReportTypeId == viper.GetInt("ReportTypeId") &&
				report.StateCode == viper.GetString("ReportStateCode") &&
				report.IsPublic != true {

				log.Print("Got new report")
				log.Printf("  / reportId: %d, reportType: %s, stateCode: %s", report.Id, report.ReportTypeName, report.StateCode)
				log.Printf("  / Address Text: %s", report.AdministrationAreaAddress)

				submit(report, client)
			}
		}
	}()

	wg.Wait()
}

func submit(report Report, client *redis.Client) {
	log.Print("Submitting...")

	report_id := report.Id

	var messageBody bytes.Buffer
	err := tmpl.Execute(&messageBody, report)
	if err != nil {
		log.Print("Fail: Can not make a template, skip.", err)
		return
	}

	// Find all user devices, except reporter's.
	var rows *sql.Rows
	rows, err = db.Query(`
		SELECT gcm_reg_id, apns_reg_id
		FROM accounts_userdevice ud,
		     accounts_authority_users au,
		     reports_administrationarea aa,
		     reports_report r
		WHERE ud.user_id = au.user_id AND
		      au.authority_id = aa.authority_id AND
		      aa.id = r.administration_area_id AND
		      ud.user_id <> $1 AND
		      r.id = $2
	`, report.CreatedById, report_id)

	if err != nil {
		log.Print("Error: Can not get user devices", err, "... skip.")
		return
	}

	defer rows.Close()

	var gcmRegIds []string
	var apnsRegIds []string
	for rows.Next() {
		var gcmRegId string
		var apnsRegId string
		err := rows.Scan(&gcmRegId, &apnsRegId)
		if err != nil {
			log.Print("Error: ", err, "... skip.")
			break
		}

		if gcmRegId != "" {
			gcmRegIds = append(gcmRegIds, gcmRegId)
		}
		if apnsRegId != "" {
			apnsRegIds = append(apnsRegIds, apnsRegId)
		}
	}

	var redisMessage RedisMessage
	redisMessage.GcmApiKey = viper.GetString("GCM_API_KEY")
	redisMessage.Type = "NEWS"
	redisMessage.Message = messageBody.String()
	redisMessage.ReportId = int64(report_id)

	var encodedMessage []byte

	// Android first.
	if len(gcmRegIds) > 0 {
		redisMessage.AndroidRegIds = gcmRegIds
		redisMessage.ApnsRegIds = []string{}

		encodedMessage, err = json.Marshal(redisMessage)
		if err != nil {
			log.Print("Error: ", err, "... skip.")
			return
		}

		client.Publish("news:new", string(encodedMessage))
	}

	// Then iOS.
	if len(apnsRegIds) > 0 {
		redisMessage.AndroidRegIds = []string{}
		redisMessage.ApnsRegIds = apnsRegIds

		encodedMessage, err = json.Marshal(redisMessage)
		if err != nil {
			log.Print("Error: ", err, "... skip.")
			return
		}

		client.Publish("news:new", string(encodedMessage))
	}

	log.Print("Done.")
}