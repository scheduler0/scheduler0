package utils

import (
	"fmt"
	"time"
)

type LogWriter struct{}

func (writer LogWriter) Write(bytes []byte) (int, error) {
	fmt.Println("----------------------------------------------------------------")
	return fmt.Print(time.Now().UTC().Format(time.RFC1123) + " [DEBUG] " + string(bytes))
}
