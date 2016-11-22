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
)

var (
	keyFlag = flag.String("key", "1234567890123456", "Key to encrypt/decrypt")
	nonceFlag = flag.String("nonce", "3a0117f29cd4261bab54b0f1", "Nonce")
	redisHostFlag = flag.String("redis.host", "127.0.0.1", "Redis host")
	redisPortFlag = flag.Int("redis.port", 6379, "Redis port")
	poddAPIURL = flag.String("api.url", "http://localhost:8000/reports/", "PODD API URL")
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
  "guid": "%s",
  "negative": false
}
`

type ZeroReportCallback struct {}
func (c ZeroReportCallback) Execute(payload PoddService.Payload) bool {
	client := &http.Client{}

	date := time.Now().Local()
	zeroReportJSON := fmt.Sprintf(zeroReport, date.Format("2006-01-02"), date.Format(time.RFC3339), date.Unix(), payload.RefNo)

	req, err := http.NewRequest("POST", *poddAPIURL, strings.NewReader(zeroReportJSON))
	if err != nil {
		fmt.Println(err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Token " + payload.Token)

	resp, err := client.Do(req)
	fmt.Println(resp)
	return resp.StatusCode == http.StatusCreated && err == nil
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

func main() {
	iniflags.Parse()

	conn, err := redis.Dial("tcp", fmt.Sprintf("%s:%d", *redisHostFlag, *redisPortFlag))
	if err != nil {
		panic(err)
	}

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

	http.HandleFunc("/report/zero/", server.ZeroReportHandler(ZeroReportCallback{}))
	http.ListenAndServe(":9800", nil)
}