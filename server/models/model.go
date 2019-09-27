package models

// Basic model interface
type Model interface {
	SetId(id string)
	CreateOne() (string, error)
	GetOne(string, interface{}) error
	GetAll(string, interface{}) ([]interface{}, error)
	UpdateOne() error
	DeleteOne() (int, error)
	FromJson(body []byte)
	ToJson() []byte
}
