package main

import (
	"os"
	"flag"
)

const chunkSize = 100
const defaultDSN = "user=postgres password=postgres dbname=postgres host=localhost port=5432 sslmode=disable"
const defaultGCMAPIKey = "YOUR_GCM_API_KEY"

var dsn *string
var gcmApiKey *string
var nonce *string
var sharedKey *string
var returnServerUrl *string

func init() {
	dsn := flag.String("dsn", defaultDSN, "DSN string, or use environment variable FRIDAYNOTICE_DSN")
	gcmApiKey := flag.String("gcmApiKey", defaultGCMAPIKey, "GCM API key, or use environment variable FRIDAYNOTICE_GCM_API_KEY")
	nonce = flag.String("nonce", "", "Nonce")
	sharedKey = flag.String("sharedKey", "SHARED_KEY", "Shared key")
	returnServerUrl = flag.String("returnServerUrl", "http://localhost:9110/report/zero/", "Return server url")

	flag.Parse()

	if *dsn == defaultDSN && os.Getenv("FRIDAYNOTICE_DSN") != "" {
		*dsn = os.Getenv("FRIDAYNOTICE_DSN")
	}

	if *gcmApiKey == defaultGCMAPIKey && os.Getenv("FRIDAYNOTICE_GCM_API_KEY") != "" {
		*gcmApiKey = os.Getenv("FRIDAYNOTICE_GCM_API_KEY")
	}

	SetDSN(dsn)
	SetGcmApiKey(gcmApiKey)
}

func SetDSN(customDSN *string) {
	dsn = customDSN
}

func SetGcmApiKey(customGcmApiKey *string) {
	gcmApiKey = customGcmApiKey
}

func main() {
	users := GetVolunteers()

	if gcmApiKey == nil || *gcmApiKey == "" {
		println("Error: Required GCM API Key")
		os.Exit(0)
	}

	sender := NewSender(*gcmApiKey)
	for _, user := range users {
		SendNotificationToUser(sender, user)
	}
}