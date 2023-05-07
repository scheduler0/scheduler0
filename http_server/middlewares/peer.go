package middlewares

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"
	"scheduler0/constants/headers"
	"scheduler0/secrets"
)

func IsPeerClient(req *http.Request) bool {
	peerHeaderVal := req.Header.Get(headers.PeerHeader)
	return peerHeaderVal == "cmd" || peerHeaderVal == "peer"
}

func IsAuthorizedPeerClient(req *http.Request, scheduler0Secrets secrets.Scheduler0Secrets) bool {
	credentials := scheduler0Secrets.GetSecrets()
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
