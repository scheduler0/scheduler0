package models

type Response struct {
	Data  []interface{} `json:"data"`
	Error string        `json:"error"`
}
