package main

import (
	"fmt"
	"log"
	"os"

	"github.com/bwmarrin/discordgo"
	// "log"
	//"os"
	"io/ioutil"
	"net/http"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const roleBanned = "Banned"

var (
	messageContents chan *discordgo.MessageCreate = make(chan *discordgo.MessageCreate)
	dgv             *discordgo.VoiceConnection
	roleBannedId    string
	adminIds        map[string]int
	quit            bool = false
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var err error

	logf, err := os.OpenFile("bot-erros.log",
		os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	defer logf.Close()
	log.SetOutput(logf)
	pgDb := dbConn()
	rows, err := pgDb.Query(
		`SELECT role_id, role FROM roles
		WHERE role IN ('Mods', 'Admin', 'Banned') `)
	checkErr(err)

	adminIds = make(map[string]int)
	for rows.Next() {
		var id string
		var role string
		rows.Scan(&id, &role)
		if role == "Banned" {
			roleBannedId = id
		} else {
			adminIds[id] = 1
		}
	}

	rows.Close()
	pgDb.Close()

	token := ""

	dg, err := discordgo.New(token)
	checkErr(err)

	dg.AddHandler(messageCreate)
	// dg.AddHandler(voiceServerUpdate) //USELESS right now.
	dg.Open()

	go func() {
		for {
			for !pullFromReddit(dg) {
			}
			time.Sleep(time.Minute * 3)
		}
	}()

	go checkBans(dg)
	go storeMessageContent()

	for {
		if quit {
			break
		}
		runtime.GC()
		debug.FreeOSMemory()
		time.Sleep(time.Second * 2)
	}

}

func storeMessageContent() {
	pgDb := dbConn()
	defer pgDb.Close()

	tx, err := pgDb.Begin()
	checkErr(err)

	ticker := time.NewTicker(time.Minute * 5)

	for {
		select {
		case contents := <-messageContents:
			_, err := tx.Exec(`INSERT INTO messages(
				id, time_stamp, message, author, channel_id)
			 VALUES($1, $2, $3, $4, $5)`, contents.ID, contents.Timestamp,
				contents.Content, contents.Author.Username, contents.ChannelID)
			checkErr(err)
		case <-ticker.C:
			tx.Commit()
			tx, err = pgDb.Begin()
			checkErr(err)
		}
	}
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	chanl, err := s.Channel(m.ChannelID)
	if err != nil {
		return
	}

	guild, _ := s.Guild(chanl.GuildID)
	var author *discordgo.Member

	if guild != nil {
		author, _ = s.GuildMember(guild.ID, m.Author.ID)
	}

	checkLink(s, m.Content, chanl.ID, chanl.LastMessageID)

	messageContents <- m

	if len(m.Content) > 21 && len(m.Mentions) > 0 &&
		m.Mentions[len(m.Mentions)-1].Username == "MrDestructoid" {
		arr := strings.Fields(m.Content)
		switch arr[1] {
		case "BD":
			bdLinks(s, m.ChannelID)
		// case "state":
		// for _, i := range guild.VoiceStates {
		// fmt.Println(i.UserID)
		// }

		// case "bot":
		// var mem runtime.MemStats
		// runtime.ReadMemStats(&mem)
		// fmt.Println(mem.HeapAlloc)

		case "myid":
			s.ChannelMessageSend(m.ChannelID,
				fmt.Sprintf("```\n%v\n```", m.Author.ID))
		// //---------------- NO WAY TO UTILIZE -----------
		// case "joinchannel":
		// ch := make(chan string)
		// fmt.Println(strings.Join(arr[2:], " "))
		// if findNjoinChannel(s, guild.ID, guild.Channels,
		// strings.Join(arr[2:], " "), ch) {
		// echo(dgv, ch)
		// fmt.Println("opened")
		// }
		// //---------------- NO WAY TO UTILIZE ----------

		case "channels":
			var str string = fmt.Sprintf("\n%-20s\t%-5s\tID", "Name", "Type")
			for _, c := range guild.Channels {
				str += fmt.Sprintf("\n%-20s\t%-5s\t%s", c.Name, c.Type, c.ID)
			}
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("```%s```", str))

		case "updateroles":
			updateRolesToDB(s, guild.ID)

		case "guild":
			var owner string
			for _, i := range guild.Members {
				if i.User.ID == guild.OwnerID {
					owner = i.User.Username
				}
			}
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
				"```**Guild information** \nName: %s\nOwner: %s\nID: %s\nRegion: %s\nHas members: %d```",
				guild.Name, owner, guild.ID, guild.Region, len(guild.Members)))

		case "quit":
			if checkForPermissions(s, chanl.ID, author.Roles) {
				quit = true
			}

		case "ban":
			banUser(s, guild, author, m.ChannelID, arr)

		case "unban":
			manualUnban(s, chanl.ID, author.Roles, arr)

		case "bannedusers":
			bannedUsers(s, m)

		default:
			s.ChannelMessageSend(chanl.ID, fmt.Sprintf("No such command"))
		}

	}
}
func bdLinks(s *discordgo.Session, id string) {
	resp, err := http.Get("https://betterdiscord.net/home/")
	checkErr(err)
	bytes, err := ioutil.ReadAll(resp.Body)
	checkErr(err)
	rx := regexp.MustCompile(`<a href="(.*.zip)`)
	mm := rx.FindAllStringSubmatch(string(bytes), 2)
	s.ChannelMessageSend(id,
		fmt.Sprintf("\n`OSX:` %s\n`Windows:` %s", mm[1][1], mm[0][1]))

}
func bannedUsers(s *discordgo.Session, m *discordgo.MessageCreate) {
	pgDb := dbConn()
	rows, err := pgDb.Query("SELECT name, time_stamp, duration from bans")
	checkErr(err)

	for rows.Next() {
		var time_stamp, duration int64
		var name string
		rows.Scan(&name, &time_stamp, &duration)
		s.ChannelMessageSend(m.ChannelID,
			fmt.Sprintf("```\n%s %s\n```", name,
				time.Unix(time_stamp+duration, 0).Format("2006-01-02 15:04")))
	}
	rows.Close()
	pgDb.Close()
}

func banUser(s *discordgo.Session, guild *discordgo.Guild,
	author *discordgo.Member, channelID string, arr []string) {
	if len(arr) == 4 && len(arr[2]) == 21 &&
		checkForPermissions(s, channelID, author.Roles) {
		arr[2] = arr[2][2 : len(arr[2])-1]
		userRole := ""
		username := ""
		for _, member := range guild.Members {
			if arr[2] == member.User.ID {
				username = member.User.Username
				userRole = member.Roles[0]
				break
			}
		}
		duration, _ := (strconv.Atoi(arr[3]))

		pgDb := dbConn()
		tx, err := pgDb.Begin()
		checkErr(err)

		rows, err := tx.Query(
			`SELECT duration FROM bans 
			WHERE user_id = $1`, arr[2])
		checkErr(err)

		var (
			dur int
			i   int
		)

		for rows.Next() {
			i++
			rows.Scan(&dur)
		}
		rows.Close()
		if i == 0 {

			_, err = tx.Exec(
				`INSERT INTO bans(name, time_stamp, duration,
				guild_id, role_id, user_id)
				VALUES($1, $2, $3, $4, $5, $6)`,
				username, int64(time.Now().Unix()),
				duration*60, guild.ID, userRole, arr[2])
			checkErr(err)

			s.GuildMemberEdit(guild.ID, arr[2], []string{roleBannedId})
			for _, c := range guild.Channels {
				if c.Name == "AFK" {
					s.GuildMemberMove(guild.ID, arr[2], c.ID)
					break
				}
			}
			s.ChannelMessageSend(channelID,
				fmt.Sprintf(
					"User %s has been temporary banned for %d minute(s)",
					username, duration))
		} else {
			_, err = tx.Exec(
				`UPDATE bans SET duration = $1
				WHERE user_id = $2`, dur+duration*60, arr[2])
			checkErr(err)

			s.ChannelMessageSend(channelID,
				fmt.Sprintf(
					"Users %s temporary ban has been extended by %d minute(s)",
					username, duration))
		}
		tx.Commit()
		pgDb.Close()
	} else {
		s.ChannelMessageSend(channelID,
			fmt.Sprintf("Please check the parameters"))
	}
}

func checkLink(s *discordgo.Session, m string, chanID string, mID string) {
	if strings.Contains(strings.ToLower(m), "www.facebook") {
		s.ChannelMessageDelete(chanID, mID)
	}
}

func checkForPermissions(s *discordgo.Session,
	channelID string, roleID []string) bool {
	var ok bool
	if len(roleID) > 0 {
		_, ok = adminIds[roleID[0]]
		if ok == true {
			return ok
		}
	}
	s.ChannelMessageSend(channelID, fmt.Sprintf("Insuficient permissions"))
	return ok

}

func updateRolesToDB(s *discordgo.Session, guild string) {
	roles, _ := s.GuildRoles(guild)
	pgDb := dbConn()
	tx, err := pgDb.Begin()
	checkErr(err)

	for i := 0; i < len(roles); i++ {
		_, err = tx.Exec("INSERT INTO roles(role, role_id) VALUES($1, $2)",
			roles[i].Name, roles[i].ID)
		checkErr(err)
	}
	tx.Commit()
	pgDb.Close()
}

func manualUnban(s *discordgo.Session, channelID string,
	roleID []string, arr []string) {

	if checkForPermissions(s, channelID, roleID) &&
		len(arr) == 3 && len(arr[2]) == 21 {
		arr[2] = arr[2][2 : len(arr[2])-1]
		pgDb := dbConn()
		tx, err := pgDb.Begin()
		checkErr(err)

		user, _ := s.User(arr[2])
		name := user.Username

		rows, err := tx.Query(
			"SELECT id, guild_id, role_id FROM bans WHERE name = $1", name)
		checkErr(err)

		flag := false
		for rows.Next() {
			flag = true
			var id, guild_id, role_id string
			rows.Scan(&id, &guild_id, &role_id)

			if len(guild_id) != 0 {
				rows.Close()
				_, err = tx.Exec("DELETE FROM bans WHERE id = $1", id)
				checkErr(err)
				s.GuildMemberEdit(guild_id, arr[2], []string{role_id})
				s.ChannelMessageSend(channelID,
					fmt.Sprintf("User %s has been unbanned", name))
				break
			}
		}
		tx.Commit()
		pgDb.Close()
		if flag == false {
			s.ChannelMessageSend(channelID,
				fmt.Sprintf("User %s is not banned", name))
		}

	} else {
		s.ChannelMessageSend(channelID,
			fmt.Sprintf("Please check the parameters"))
	}
}
func checkBans(s *discordgo.Session) {
	for {
		time.Sleep(10 * time.Second)
		currentTs := int64(time.Now().Unix())

		pgDb := dbConn()
		tx, err := pgDb.Begin()
		checkErr(err)

		rows, err := tx.Query(`SELECT id, name, guild_id, role_id, user_id 
		FROM bans WHERE (time_stamp + duration) <= $1`, currentTs)
		checkErr(err)

		var id, name, guild_id, role_id, user_id string
		for rows.Next() {

			rows.Scan(&id, &name, &guild_id, &role_id, &user_id)

			if id != "" && name != "" && guild_id != "" &&
				role_id != "" && user_id != "" {
				rows.Close()
				fmt.Println("passed")
				fmt.Println(id, name, guild_id, role_id, user_id)
				_, err := tx.Exec("DELETE FROM bans WHERE id = $1", id)
				checkErr(err)

				s.GuildMemberEdit(guild_id, user_id, []string{role_id})
				break
			}

		}

		tx.Commit()
		pgDb.Close()

	}
}

// // //---------------- NO WAY TO UTILZE ------------------------------
// func echo(v *discordgo.VoiceConnection, ch chan string) {
// // chanID := <-ch
// v.AddHandler(func(vc *discordgo.VoiceConnection,
// vs *discordgo.VoiceSpeakingUpdate) {
// recv := make(chan *discordgo.Packet, 2)
// go dgvoice.ReceivePCM(v, recv)
// v.Speaking(true)
// defer v.Speaking(false)
// var totalSum, totalLen int64 = 0, 0
// for {
// p, ok := <-recv
// // fmt.Println(p.SSRC)
// var packetSum, packetLen int64 = 0, 0
// for _, i := range p.PCM {
// if i < 0 {
// i *= -1
// }
// packetSum += int64(i)
// packetLen++
// }
// // fmt.Printf("Packetsum: %v\nPacketLen: %v\n",packetSum, packetLen)
// if packetSum != 0 {
// // fmt.Printf("totalsum: %v\ntotalLen: %v\n", totalSum, totalLen)
// totalSum += packetSum
// totalLen += packetLen
// // fmt.Printf("total: %v\n", totalSum/totalLen)
// } else if totalSum != 0 {
// fmt.Printf("total: %v\n", totalSum/totalLen)
// totalSum, totalLen = 0, 0
// }
// if !ok {
// return
// }
// }
// })
// }

// // func voiceServerUpdate(s *discordgo.Session, v *discordgo.VoiceServerUpdate) {
// // 	time.Sleep(time.Millisecond * 100)
// // 	echo(dgv)
// // }

// func findNjoinChannel(s *discordgo.Session, guildID string,
// channels []*discordgo.Channel, name string, ch chan string) bool {
// var cID string
// for _, c := range channels {
// if c.Name == name {
// cID = c.ID
// break
// }
// }
// fmt.Println("Before opening voice channel")
// dgv, _ = s.ChannelVoiceJoin(guildID, cID, true, false)
// time.Sleep(time.Second * 2)
// // dgv, _ = s.ChannelVoiceJoin("103598153973895168", "103600827821735936", true, false)
// // checkErr(err)
// ch <- cID
// fmt.Println("Joined voice channel")

// // err =
// // dgv.WaitUntilConnected()
// // checkErr(err)

// return true
// }

// //---------------- NO WAY TO UTILZE ------------------------------
