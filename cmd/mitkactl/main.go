package main

import (
	"flag"
	"log"

	"github.com/molchalin/mitkabot/internal/config"
	"github.com/molchalin/mitkabot/internal/poll"
)

func requirePoll(cfg *config.Config) {
	if cfg.PollFile == "" {
		log.Fatalf("poll_file required")
	}
}

func requireBooksDB(cfg *config.Config) {
	if cfg.BookDB == "" {
		log.Fatalf("book_db required")
	}
}

func requireResultDB(cfg *config.Config) {
	if cfg.ResultDB == "" {
		log.Fatalf("result_db required")
	}
}

func main() {
	cfg, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	switch flag.Args()[0] {
	case "mk":
		requirePoll(cfg)
		requireBooksDB(cfg)
		requireResultDB(cfg)

		err := poll.CreatePollFromNotion(cfg)
		if err != nil {
			log.Fatal(err)
		}
	case "push":
		err := poll.PushResult(cfg)
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatalf("unknown tool: %v", flag.Args()[0])
	}
}
