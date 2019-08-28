package main

import (
	"bufio"
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func requestWorker(domains chan string, wg *sync.WaitGroup, db *sql.DB) {
	defer wg.Done()
	//defer db.Close()

	//use this to disable security check validation
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	httpClient := &http.Client{Transport: tr}

	//httpClient := &http.Client{}

	for domain := range domains {
		response, err := httpClient.Head(domain)

		var statusCode int

		if err == nil {
			statusCode = response.StatusCode
			log.Println(fmt.Sprintf("[%s] %d", domain, response.StatusCode))
		} else {
			statusCode = -1
			log.Println(fmt.Sprintf("[%s] HTTP Error %s", domain, err.Error()))
		}

		//log.Println(fmt.Sprintf("INSERT INTO sites(id, domain, code) values(\"%s\", %d)", domain, statusCode))
		_, err = db.Exec(fmt.Sprintf("INSERT INTO sites(domain, code, time) values(\"%s\", %d, \"%s\")", domain, statusCode, time.Now()))
		if err != nil {
			log.Fatalln("could not insert row:", err)
		}

	}
}

func initDataBase() *sql.DB {
	db, err := sql.Open("sqlite3", "./sites.db")
	if err != nil {
		log.Fatal(err)
	}

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name='sites'")
	if err != nil {
		log.Panic("%q: \n", err)
		os.Exit(1)
	}
	defer rows.Close()
	var name string

	for rows.Next() {
		rows.Scan(&name)
		//log.Println(name)
	}

	if name != "sites" {

		sqlStmt := `
			DELETE FROM sites;
			DROP TABLE sites;
			`
		db.Exec(sqlStmt)
		if err != nil {
			log.Panic("%q: %s\n", err, sqlStmt)
			os.Exit(1)
		}

		sqlStmt = `
			CREATE TABLE sites (site_id INTEGER PRIMARY KEY AUTOINCREMENT, domain TEXT NOT NULL, code INTEGER, time TEXT NOT NULL);
			`
		_, err = db.Exec(sqlStmt)
		if err != nil {
			log.Panic("%q: %s\n", err, sqlStmt)
			os.Exit(1)
		}
	}

	return db
}
func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	var threadsCount int
	flag.IntVar(&threadsCount, "threads", 15, "Count of used threads (goroutines)")

	file, err := os.Open("domains_tc")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	taskChannel := make(chan string, 10000)
	go func() {
		for scanner.Scan() {
			domain := fmt.Sprintf("%s", scanner.Text())
			taskChannel <- domain
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}

		close(taskChannel)
	}()

	db := initDataBase()

	wg := new(sync.WaitGroup)

	for i := 0; i < threadsCount; i++ {
		wg.Add(1)
		go requestWorker(taskChannel, wg, db)
	}

	wg.Wait()
	db.Close()
}
