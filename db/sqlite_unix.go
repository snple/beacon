//go:build linux || darwin
// +build linux darwin

package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func dbOpen(file string) (*sql.DB, error) {
	return sql.Open("sqlite",
		fmt.Sprintf("file:%v?_pragma=cache_size(16000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", file))
}
