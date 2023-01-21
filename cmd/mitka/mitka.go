package main

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"

	"github.com/molchalin/mitkabot/internal/config"
	"github.com/molchalin/mitkabot/internal/handler"
	"github.com/molchalin/mitkabot/internal/poll"
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	p, err := poll.NewPoll(cfg)
	if err != nil {
		log.Fatal(err)
	}

	d := handler.NewDispatcher(p)
	bot, err := tgbotapi.NewBotAPI(cfg.TgToken)
	if err != nil {
		log.Fatal(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Fatalf("error: %v", err)
	}

	type msgID struct {
		chatID int64
		ID     int
		olo    string
	}
	msgs := make(map[string]msgID)

	for update := range updates {
		var forceNewMsg bool
		var username string
		if update.CallbackQuery == nil && update.Message == nil {
			continue
		}
		var chatID int64
		if update.CallbackQuery != nil {
			username = update.CallbackQuery.From.UserName
			chatID = update.CallbackQuery.Message.Chat.ID
			d.Handler(username, update.CallbackQuery.Data)
		} else {
			forceNewMsg = true
			username = update.Message.From.UserName
			chatID = update.Message.Chat.ID
			d.Handler(username, update.Message.Text)
		}
		if chatID == cfg.ChatID {
			continue
		}
		var mark tgbotapi.InlineKeyboardMarkup
		if bs := d.Buttons(username); len(bs) > 0 {
			mark = tgbotapi.NewInlineKeyboardMarkup(bs...)
		}

		if m, ok := msgs[username]; ok && !forceNewMsg {
			msg := tgbotapi.NewEditMessageText(m.chatID, m.ID, m.olo+" "+d.Text(username))
			msg.ReplyMarkup = &mark
			msg.ParseMode = "Markdown"
			msg.DisableWebPagePreview = true
			msgNew, err := bot.Send(msg)
			if err != nil {
				log.Fatal(err)
			}
			msgs[username] = msgID{msgNew.Chat.ID, msgNew.MessageID, swapOlo(m.olo)}
		} else {
			m.olo = swapOlo(m.olo)
			msg := tgbotapi.NewMessage(chatID, m.olo+" "+d.Text(username))
			msg.DisableWebPagePreview = true
			msg.ParseMode = "Markdown"
			msg.ReplyMarkup = &mark
			msgNew, err := bot.Send(msg)
			if err != nil {
				log.Fatal(err)
			}
			if ok {
				_, err := bot.DeleteMessage(tgbotapi.NewDeleteMessage(m.chatID, m.ID))
				if err != nil {
					log.Fatal(err)
				}
			}
			msgs[username] = msgID{msgNew.Chat.ID, msgNew.MessageID, swapOlo(m.olo)}
		}
	}
}

func swapOlo(str string) string {
	if str == "📗" {
		return "📘"
	}
	return "📗"
}
