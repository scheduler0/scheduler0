package utils

import "fmt"

type GenericError struct {
	Message string
	Type    int
}

func (g *GenericError) Error() string {
	return fmt.Sprintf("message: %s, code: %v", g.Message, g.Type)
}

func HTTPGenericError(httpStatus int, errorMessage string) *GenericError {
	return &GenericError{
		Type:    httpStatus,
		Message: errorMessage,
	}
}
