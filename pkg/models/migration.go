package models

import "database/sql"

type Migration struct {
	Name string
	Up   func(db *sql.Tx) error
	Down func(db *sql.Tx) error
}
