package main

import (
	"os"
	"flag"
	"github.com/vharitonsky/iniflags"
	"strings"
)

const chunkSize = 100
const defaultDSN = "user=postgres password=postgres dbname=postgres host=localhost port=5432 sslmode=disable"
const defaultGCMAPIKey = "YOUR_GCM_API_KEY"

var (
	dsn = flag.String("dsn", defaultDSN, "DSN string, or use environment variable FRIDAYNOTICE_DSN")
	gcmApiKey = flag.String("gcmApiKey", defaultGCMAPIKey, "GCM API key, or use environment variable FRIDAYNOTICE_GCM_API_KEY")
	nonce = flag.String("nonce", "", "Nonce")
	sharedKey = flag.String("sharedKey", "SHARED_KEY", "Shared key")
	returnServerUrl = flag.String("returnServerUrl", "http://localhost:9110/report/zero/", "Return server url")
	messagesFlag = flag.String("messages", "อาสาผ่อดีดีตรวจสอบเหตุการณ์ในพื้นที่ของตนเอง ถ้าไม่มีสิ่งใดผิดปกติ กรุณาส่งรายงานไม่พบเหตุการณ์ผิดปกติมายังโครงการผ่อดีดีด้วย ขอบคุณค่ะ", "Set of messages to send separated by ### (triple sharp)")
	debugFlag = flag.Bool("debug", false, "Debug flag")
	testUsername = flag.String("testUsername", "podd.demo", "Test username")
)

var messages []string

func init() {
	iniflags.Parse()

	messages = strings.Split(*messagesFlag, "###")

	if *dsn == defaultDSN && os.Getenv("FRIDAYNOTICE_DSN") != "" {
		*dsn = os.Getenv("FRIDAYNOTICE_DSN")
	}

	if *gcmApiKey == defaultGCMAPIKey && os.Getenv("FRIDAYNOTICE_GCM_API_KEY") != "" {
		*gcmApiKey = os.Getenv("FRIDAYNOTICE_GCM_API_KEY")
	}
}

func main() {
	var users []*User
	if *debugFlag {
		users = GetVolunteers(*testUsername)
	} else {
		users = GetVolunteers("")
	}

	if gcmApiKey == nil || *gcmApiKey == "" {
		println("Error: Required GCM API Key")
		os.Exit(0)
	}

	sender := NewSender(*gcmApiKey)
	for _, user := range users {
		SendNotificationToUser(sender, user)
	}
}