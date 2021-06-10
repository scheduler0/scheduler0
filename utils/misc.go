package utils

import (
	"os"
)

// GetPort returns the port in which the server should be running
func GetPort() string {
	port := os.Getenv("PORT")

	if len(port) == 0 {
		port = "9090"
	}

	return ":" + port
}

// GetAuthentication returns basic auth used to for identifying request from the dashboard
func GetAuthentication() (string, string) {
	usernameEnv := os.Getenv("username")
	passwordEnv := os.Getenv("password")

	username := "admin"
	password := "admin"

	if len(usernameEnv) == 0 {
		username += usernameEnv
	}

	if len(passwordEnv) == 0 {
		password += passwordEnv
	}

	return username, password
}
