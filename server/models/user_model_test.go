package models

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestUser_CreateOne(t *testing.T) {
	t.Log("Create Password Hash")
	{
		password := "password"
		hash := sha256.New()
		hash.Write([]byte(password))
		hashStr := hex.EncodeToString(hash.Sum(nil))

		fmt.Println(password, hashStr)
	}
}
