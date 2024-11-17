package models

type Work struct {
	SuccessChannel chan any
	ErrorChannel   chan any
	Effector       func(successChannel chan any, errorChannel chan any)
}
