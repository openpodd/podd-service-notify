package podd_service_notify

import (
	"testing"
	"crypto/aes"
	"fmt"
	"bytes"
	"io"
	"crypto/rand"
	"encoding/hex"
	"crypto/cipher"
	"strings"
	"time"
)

func TestEncryptSimpleText(t *testing.T) {
	key := []byte("1234567890123456")
	text := []byte(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Log("Fail new cipher block", err)
		t.FailNow()
	}

	nonce, err := hex.DecodeString("3a0117f29cd4261bab54b0f1")
	if err != nil {
		t.Log("Fail decode nonce string")
		t.FailNow()
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Log("Fail aesgcm")
		t.FailNow()
	}

	cipherText := aesgcm.Seal(nil, nonce, text, nil)
	fmt.Printf("Cipher text is %x with size is %d\n", cipherText, len(cipherText))

	// decrypt
	plainText, err := aesgcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		t.Log("Fail decrypt seal")
		t.FailNow()
	}
	fmt.Printf("Plain text is %s\n", plainText)

	if bytes.Compare(text, plainText) != 0 {
		t.Log("Encrypted != Decrypted")
		t.FailNow()
	}
}

func TestEncrypt(t *testing.T) {
	key := "1234567890123456"
	text := key

	c := Cipher{
		Key: key,
		Nonce: "3a0117f29cd4261bab54b0f1",
	}

	encrypted, _ := c.Encrypt(text)
	if strings.Compare("b3f190cec40027680566823f6a8b2ff314719ce2bbc801d016764ae22b0e5a3e", encrypted) != 0 {
		t.Log("Encrypted is not expected", encrypted)
		t.FailNow()
	}
}

func TestDecrypt(t *testing.T) {
	key := "1234567890123456"
	encryptedText := "b3f190cec40027680566823f6a8b2ff314719ce2bbc801d016764ae22b0e5a3e"

	c := Cipher{
		Key: key,
		Nonce: "3a0117f29cd4261bab54b0f1",
	}

	decrypted, err := c.Decrypt(encryptedText)
	if err != nil {
		t.Log("Err", err)
		t.FailNow()
	}
	if strings.Compare("1234567890123456", decrypted) != 0 {
		t.Log("Decrypted is not expected", decrypted)
		t.FailNow()
	}
}

func TestNounce(t *testing.T) {
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		t.Error(err.Error())
	}

	stringNonce := hex.EncodeToString(nonce)
	fmt.Println(stringNonce)

	if n, err := hex.DecodeString(stringNonce); err != nil || bytes.Compare(n, nonce) != 0 {
		t.Log("Encoded nonce is not equal to decoded nonce")
		t.FailNow()
	}
}

func TestEncodePayload(t *testing.T) {
	key := "1234567890123456"

	c := Cipher{
		Key: key,
		Nonce: "3a0117f29cd4261bab54b0f1",
	}

	payload, err := CreatePayload("zsbUIyuvtcAjaXqOFP2ImE3_XsIcSO96y8qPRRFDlAzAdwjTzZA5ekkSEj3tMoeT", 123456, time.Second * 1)
	if err != nil {
		t.Log("Cannot create payload", err)
		t.FailNow()
	}

	encoded, err := c.EncodePayload(payload)
	if err != nil {
		t.Log("Cannot encode payload", err)
		t.FailNow()
	}

	payload, err = c.DecodePayload(encoded)
	if err != nil {
		t.Log("Cannot decode payload", err)
		t.FailNow()
	}

	if strings.Compare(payload.Token, "zsbUIyuvtcAjaXqOFP2ImE3_XsIcSO96y8qPRRFDlAzAdwjTzZA5ekkSEj3tMoeT") != 0 {
		t.Log("Token is not correct")
		t.FailNow()
	}
}

func TestPayload_IsExpired(t *testing.T) {
	payload, _ := CreatePayload("1234", 1234, -1 * time.Second)

	if !payload.IsExpired() {
		t.Log("Payload must be expired")
		t.FailNow()
	}

	payload, _ = CreatePayload("1234", 1234, 10 * time.Second)

	if payload.IsExpired() {
		t.Log("Payload must not be expired")
		t.FailNow()
	}
}