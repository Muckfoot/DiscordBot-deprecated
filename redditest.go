package main

import (
	"bitbucket.org/liamstask/go-imgur/imgur"
	// "database/sql"
	// "fmt"
	"github.com/bwmarrin/discordgo"
	// "github.com/jzelinskie/geddit"
	// _ "github.com/lib/pq"
	// _ "github.com/mattn/go-sqlite3"
	// "io"
	// "io/ioutil"
	"net/http"
	// "os"
	// "strings"
	// "time"
	// "log"
)

func pullFromReddit(ses *discordgo.Session) bool {
	response, err := http.Get("http://i.imgur.com/PCE7BVt.gif")
	checkErr(err)
	// fmt.Println("GET")
	// content, err := ioutil.ReadAll(response.Body)
	// checkErr(err)
	// fmt.Println("READ")
	// ioutil.WriteFile("tmp.gif", content, 0644)
	// fmt.Println("WRITE")
	// file, err := os.Open("tmp.gif")
	// checkErr(err)
	// fmt.Println("OPEN")
	// size := response.ContentLength / 1024 / 1024
	// if size > 8{

	// }
	ses.ChannelFileSend("155359264557236224", "tmp.gif", response.Body)
	return true
}
func imgurFilext(id string) string {
	client := imgur.NewClient(nil, "a9ffc6bf280773c", "733f0c0e32d8ed156f8bf52befca0aaa1ef957ab")
	s := client.Image
	// fmt.Println(s)
	link, err := s.Info(id)
	checkErr(err)
	// fmt.Println(link.Link)
	return link.Link
}
