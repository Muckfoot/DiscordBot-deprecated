package main

import (
	"database/sql"
	"log"
	"runtime"

	_ "github.com/lib/pq"
	// "strings"
)

func dbConn() *sql.DB {
	pgDb, err := sql.Open("postgres", "postgres://postgres@192.168.1.77/data?sslmode=disable")
	checkErr(err)

	return pgDb
}
func checkErr(err error) {
	_, file, line, _ := runtime.Caller(1)
	if err != nil {
		log.Fatalf("file: %s, line: %d, error: %s", file, line, err)
	}
}

// func caseInsenstiveContains(a, b string) bool {
// 	return strings.Contains(strings.ToUpper(a), strings.ToUpper(b))
// }
