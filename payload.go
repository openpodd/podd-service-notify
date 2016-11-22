package podd_service_notify

import (
	"crypto/aes"
	"encoding/hex"
	"crypto/cipher"
	"fmt"
	"time"
	"strings"
	"strconv"
	"io"
	"crypto/rand"
)

type Cipher struct {
	Key string
	Nonce string
}

type Payload struct {
	Token string
	Expire time.Time
	Id int
	RefNo string
}

func (c Cipher) getGCMBlock() (cipher.AEAD, []byte, error) {
	block, err := aes.NewCipher([]byte(c.Key))
	if err != nil {
		return nil, nil, err
	}

	nonce, err := hex.DecodeString(c.Nonce)
	if err != nil {
		return nil, nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	return aesgcm, nonce, nil
}

func (c Cipher) Encrypt(text string) (string, error) {
	aesgcm, nonce, err := c.getGCMBlock()
	if err != nil {
		return "", err
	}

	cipherText := aesgcm.Seal(nil, nonce, []byte(text), nil)
	return hex.EncodeToString(cipherText), nil
}

func (c Cipher) Decrypt(encryptedText string) (string, error) {
	aesgcm, nonce, err := c.getGCMBlock()
	if err != nil {
		fmt.Println("Error aesgcm")
		return "", err
	}

	byteText, err := hex.DecodeString(encryptedText)
	if err != nil {
		fmt.Println("Error convert byte text")
		return "", err
	}

	plainText, err := aesgcm.Open(nil, nonce, byteText, nil)
	if err != nil {
		fmt.Println("Error Open")
		return "", err
	}

	return string(plainText), nil
}

func (c Cipher) EncodePayload(payload Payload) (string, error) {
	payloadStr := fmt.Sprintf("%s:%d:%d:%s", payload.Token, payload.Expire.Unix(), payload.Id, payload.RefNo)
	return c.Encrypt(payloadStr)
}

func (c Cipher) DecodePayload(payloadStr string) (Payload, error) {
	decrypted, err := c.Decrypt(payloadStr)
	if err != nil {
		return Payload{}, err
	}

	arr := strings.Split(decrypted, ":")
	token := arr[0]
	timeInt, err := strconv.ParseInt(arr[1], 10, 64)
	if err != nil {
		return Payload{}, err
	}

	expire := time.Unix(timeInt, 0)
	id, err := strconv.Atoi(arr[2])
	if err != nil {
		return Payload{}, err
	}

	return Payload{
		Token: token,
		Expire: expire,
		Id: id,
		RefNo: arr[3],
	}, nil
}

func CreatePayload(token string, id int, d time.Duration) (Payload, error) {
	now := time.Now()
	expire := now.Add(d)

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return Payload{}, err
	}

	return Payload{
		Token: token,
		Expire: expire,
		Id: id,
		RefNo: hex.EncodeToString(nonce),
	}, nil
}

func (p Payload) IsExpired() bool {
	now := time.Now()
	return now.After(p.Expire)
}
