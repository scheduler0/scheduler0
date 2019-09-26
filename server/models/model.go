package models

// Basic model interface
type Model interface {
	SetId(id string)
	CreateOne() (string, error)
	GetOne() error
	GetAll() (interface{}, error)
	UpdateOne() error
	DeleteOne() error
	FromJson(body []byte)
	ToJson() []byte
}
