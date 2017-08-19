package main

// WARNING! WARNING!
// Don't forget to say botfather DISABLE over /setprivacy, so bot will see all messages in group

import (
	"encoding/binary"
	"fmt"
	"log"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/peterbourgon/diskv"
)

// Config variables
type Config struct {
	Token      string
	Secretkey  string
	WelcomeMsg string `toml:"WelcomeMessage"`
	Debug      int
}

var conf Config
var d *diskv.Diskv
var usercache map[string]uint32

func processMessage(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	var userstatevalue uint32
	// debug
	log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

	// Is it join message?
	if update.Message.NewChatMembers != nil {
		var username string
		var ok bool
		userstatekey := fmt.Sprintf("userstate_%d", (*update.Message.NewChatMembers)[0].ID)

		if userstatevalue, ok = usercache[userstatekey]; !ok {
			userstatevalueBinary, err := d.Read(userstatekey)
			if err != nil {
				// Not found in db and cache - means newcomer
				userstatevalue = 0
				usercache[userstatekey] = userstatevalue
			} else {
				// Not in cache, but in db
				userstatevalue = binary.LittleEndian.Uint32(userstatevalueBinary)
				usercache[userstatekey] = userstatevalue
			}
		} else {
			// User found in cache
		}

		// If user state is 1 (validated member) or 2 (old member) - return prematurely
		// TODO: Welcome validated members, and maybe ask old members to revalidate?
		if userstatevalue == 1 || userstatevalue == 2 {
			return
		}
		//log.Println(userstatevalue)

		if (*update.Message.NewChatMembers)[0].UserName != "" {
			username = "@" + (*update.Message.NewChatMembers)[0].UserName
		} else {
			if (*update.Message.NewChatMembers)[0].FirstName != "" || (*update.Message.NewChatMembers)[0].LastName != "" {
				username = (*update.Message.NewChatMembers)[0].FirstName + " " + (*update.Message.NewChatMembers)[0].LastName
			} else {
				username = "Totally noname (no names, no usernames)"
			}
		}
		reply := fmt.Sprintf(`Привет %s! %s`, username, conf.WelcomeMsg)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
		msg.ReplyToMessageID = update.Message.MessageID
		bot.Send(msg)

		bseq := make([]byte, 4)
		binary.LittleEndian.PutUint32(bseq, 0)
		d.Write(userstatekey, bseq)
		return
	}
	if update.Message.Text != "" {
		var ok bool
		userstatekey := fmt.Sprintf("userstate_%d", update.Message.From.ID)
		if userstatevalue, ok = usercache[userstatekey]; !ok {
			userstatevalueBinary, err := d.Read(userstatekey)
			if err != nil {
				// Not in cache, not in db, old user, set 2
				// TODO: make configurable
				bseq := make([]byte, 4)
				binary.LittleEndian.PutUint32(bseq, 2)
				d.Write(userstatekey, bseq)
				userstatevalue = 2
				usercache[userstatekey] = userstatevalue
			} else {
				// Not in cache, but in db
				userstatevalue = binary.LittleEndian.Uint32(userstatevalueBinary)
				usercache[userstatekey] = userstatevalue
			}
		} else {
			// User found in cache
		}
		if userstatevalue == 0 {
			var hasurl bool
			hasurl = false

			if strings.Contains(update.Message.Text, "#whois") {
				userstatevalue = 1
				usercache[userstatekey] = userstatevalue
				bseq := make([]byte, 4)
				binary.LittleEndian.PutUint32(bseq, 1)
				d.Write(userstatekey, bseq)

				reply := fmt.Sprintf(`Спасибо за регистрацию!`)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
				msg.ReplyToMessageID = update.Message.MessageID
				bot.Send(msg)
				return
			}
			if update.Message.Entities != nil {
				entities := *update.Message.Entities
				for _, entity := range entities {
					if entity.Type == "url" {
						hasurl = true
					}
				}
			}

			// Check if newcomer trying to post something with url or @ symbol - delete Message
			// TODO: ban after he repeats many time
			if strings.Contains(strings.ToLower(update.Message.Text), "http://") ||
				strings.Contains(strings.ToLower(update.Message.Text), "https") ||
				strings.Contains(update.Message.Text, "@") || hasurl == true {
				deleteMessageConfig := tgbotapi.DeleteMessageConfig{
					ChatID:    update.Message.Chat.ID,
					MessageID: update.Message.MessageID,
				}
				_, err := bot.DeleteMessage(deleteMessageConfig)
				log.Println(err)
			}
		}

	}

} // End of message handling function

func main() {
	if _, err := toml.DecodeFile("zenerbot.toml", &conf); err != nil {
		log.Panic(err)
	}
	flatTransform := func(s string) []string { return []string{} }
	// Initialize a new diskv store, rooted at "my-data-dir", with a 1MB cache.
	d = diskv.New(diskv.Options{
		BasePath:     "db",
		Transform:    flatTransform,
		CacheSizeMax: 1024 * 1024,
	})
	// initialize map
	usercache = make(map[string]uint32)

	// Test if it is new database or existing one
	// TODO: Maybe not needed later on, but we might use for setup wizard
	key := "_initialized"
	_, err := d.Read(key)
	if err != nil {
		// Initialize database
		d.Write(key, []byte("done"))
	}

	bot, err := tgbotapi.NewBotAPI(conf.Token)
	if err != nil {
		log.Panic(err)
	}

	if conf.Debug == 1 {
		bot.Debug = true
	}

	log.Printf("Bot authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Println(err)
	}

	for update := range updates {
		if update.Message == nil {
			continue
		}

		processMessage(bot, update)
	}
}
