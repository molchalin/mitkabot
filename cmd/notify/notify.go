package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/molchalin/mitkabot/internal/config"
	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/telegram"
)

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	telegramService, err := telegram.New(cfg.TgToken)
	if err != nil {
		log.Fatal(err)
	}

	telegramService.AddReceivers(cfg.ChatID)

	notify.UseServices(telegramService)

	tz, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		log.Fatal(err)
	}

	var cnt int
	uf := -1

	for range time.Tick(time.Minute) {
		now := time.Now().In(tz)
		hour, min, _ := now.Clock()
		if uf == -1 {
			uf = min
		}
		if min != uf {
			continue
		}
		if hour > 2 && hour < 10 {
			continue
		}
		cnt++
		if cnt%3 != 1 {
			continue
		}
		err = notify.Send(
			context.Background(),
			"Проголосуй - или Дима сделает с тобой то же самое, что и со мной",
			fmt.Sprintf("Осталось : %vh %vm", 23-hour, (60-min)%60),
		)
		if err != nil {
			log.Fatal(err)
		}
	}

}
