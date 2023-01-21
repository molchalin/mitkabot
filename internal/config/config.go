package config

import (
	"flag"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	TgToken string `yaml:"tg_token"`
	// ChatID of kkmitka group chat. Used to distuingusish messages from in the group and messages in the bot.
	ChatID int64 `yaml:"chat_id"`

	NotionToken string `yaml:"notion_token"`
	// BookDB its an ID of Notion DB where books for this month are stored.
	BookDB string `yaml:"book_db"`
	// BookDB its an ID of Notion DB where poll results for this month are stored.
	ResultDB string `yaml:"result_db"`

	// NotionTGMap stores notion name -> tg nickname mapping.
	NotionTGMap map[string]string `yaml:"notion_tg_map"`
	TGNotionMap map[string]string `yaml:"-"`

	PollFile string   `yaml:"poll_file"`
	Admins   []string `yaml:"admins"`
}

var configFile = flag.String("config", "/etc/mitka.yml", "path to config")

func Read() (*Config, error) {
	flag.Parse()

	data, err := os.ReadFile(*configFile)
	if err != nil {
		return nil, err
	}
	cfg := new(Config)
	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, err
	}
	cfg.TGNotionMap = make(map[string]string, len(cfg.NotionTGMap))
	for notion, tg := range cfg.NotionTGMap {
		cfg.TGNotionMap[tg] = notion
	}
	return cfg, nil
}
