package models

// Basic model interface
type Model interface {
	SetId(id string)
	CreateOne() (string, error)
	GetOne(string, interface{}) error
	GetAll(string, ...string) ([]interface{}, error)
	UpdateOne() error
	DeleteOne() (int, error)
	FromJson(body []byte)
	ToJson() []byte
}
