package models

type SQLResponse struct {
	RowsAffected   int64 `json:"rowsAffected"`
	LastInsertedId int64 `json:"LastInsertedId"`
}

type FSMResponse struct {
	Data  SQLResponse `json:"data"`
	Error string      `json:"error"`
}
