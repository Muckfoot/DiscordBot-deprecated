package main

import (
	//"bitbucket.org/liamstask/go-imgur/imgur"
	// "database/sql"
	"fmt"
	"io/ioutil"

	"github.com/bwmarrin/discordgo"
	"github.com/jzelinskie/geddit"
	_ "github.com/lib/pq"
	// _ "github.com/mattn/go-sqlite3"
	// "io"
	"net/http"
	// "os"
	"regexp"
	"strings"
	"time"
	// "log"
)

/*163218197003239424  -  reddit channel*/
/*155359264557236224  -  development channel*/

const CHANID = "163218197003239424"

func pullFromReddit(ses *discordgo.Session) bool {
	pgDb := dbConn()

	defer func() bool {
		if err := recover(); err != nil {
			//			fmt.Println("recovered from wrong pull")
			pgDb.Close()
			return false
		}
		return true
	}()

	session, _ := geddit.NewLoginSession(
		"",
		"",
		"gedditAgent v1",
	)

	rows, err := pgDb.Query(
		`SELECT last_url FROM reddit 
        ORDER BY time_stamp DESC LIMIT 100`)
	checkErr(err)

	var url string
	urlArr := make(map[string]int)
	for rows.Next() {
		rows.Scan(&url)
		urlArr[url] = 1
	}
	rows.Close()

	subOpts := geddit.ListingOptions{
		Limit: 100,
	}

	submissions, _ := session.Frontpage(geddit.HotSubmissions, subOpts)

	for _, s := range submissions {
		_, ok := urlArr[s.URL]
		if ok {
			break
		}
		tx, err := pgDb.Begin()
		checkErr(err)

		_, err = tx.Exec("INSERT INTO reddit (last_url) VALUES ($1)", s.URL)
		checkErr(err)

		tx.Commit()

		if strings.Contains(s.URL, "imgur") &&
			!strings.Contains(s.URL, "/a/") &&
			!strings.Contains(s.URL, "/gallery/") {
			//			fmt.Println(s.URL)
			rx := regexp.MustCompile(`^(https?://.*?)/(.*?)(\..*)?$`)
			m := rx.FindStringSubmatch(s.URL)
			//			fmt.Println(m)

			base := m[1]
			pref := m[2]
			ext := m[3]
			//			fmt.Println("after m split")
			if len(ext) == 0 {
				ext = ".jpg"
			}
			animated := imgurFilext(pref)

			url := base + "/" + pref + ext
			if animated {
				url = base + "/" + pref + ".gif"
			}

			response, err := http.Get(url)
			checkErr(err)

			//			fmt.Println(response.ContentLength)

			if response.ContentLength <= 8388608 {
				ses.ChannelMessageSend(CHANID,
					fmt.Sprintf("\n```%s```\n%s\n", s.Title, url))
			} else {
				ses.ChannelMessageSend(CHANID,
					fmt.Sprintf(
						"\n```**CANNOT EMBED FILE IS TOO LARGE**\n%s```\n%s",
						s.Title, s.URL))
			}

		} else {
			ses.ChannelMessageSend(CHANID,
				fmt.Sprintf("\n```%s```\n%s\n", s.Title, s.URL))
		}
		time.Sleep(time.Second * 2)
	}
	pgDb.Close()
	return true
}

func imgurFilext(id string) bool {
	url := "https://api.imgur.com/3/image/" + id
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	checkErr(err)
	//	fmt.Println("after request")
	req.Header.Set("Authorization", "Client-ID a9ffc6bf280773c")
	res, err := client.Do(req)
	checkErr(err)
	//	fmt.Println("after respone")
	content, err := ioutil.ReadAll(res.Body)
	checkErr(err)
	//	fmt.Println("content received")
	rx := regexp.MustCompile(`"animated":(.*?),`)
	m := rx.FindStringSubmatch(string(content))
	//	fmt.Println(m)
	//	fmt.Println(string(content))
	if m[1] == "true" {
		return true
	}

	return false
}
