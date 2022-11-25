package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"log"
	"net/http"
	"scheduler0/headers"
	"scheduler0/secrets"
)

func IsPeerClient(req *http.Request) bool {
	peerHeaderVal := req.Header.Get(headers.PeerHeader)
	return peerHeaderVal == "cmd" || peerHeaderVal == "peer"
}

func IsAuthorizedPeerClient(req *http.Request, logger *log.Logger) bool {
	credentials := secrets.GetSecrets(logger)
	username, password, ok := req.BasicAuth()
	if ok {
		usernameHash := sha256.Sum256([]byte(username))
		passwordHash := sha256.Sum256([]byte(password))

		expectedUsernameHash := sha256.Sum256([]byte(credentials.AuthUsername))
		expectedPasswordHash := sha256.Sum256([]byte(credentials.AuthPassword))

		usernameMatch := subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1
		passwordMatch := subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1

		return usernameMatch && passwordMatch
	}

	return false
}
