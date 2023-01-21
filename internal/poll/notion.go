package poll

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jomei/notionapi"
	"github.com/molchalin/mitkabot/internal/config"
)

var re = regexp.MustCompile(`\W+`)

func mustNotion(cfg *config.Config, str string) string {
	if v, ok := cfg.NotionTGMap[str]; ok {
		return v
	}
	panic("unknown user")
}

func CreatePollFromNotion(cfg *config.Config) error {
	p, err := CreatePoll(cfg.PollFile)
	if err != nil {
		return err
	}
	p.Type = TypeBook
	p.ResultDB = cfg.ResultDB
	err = fillBooks(cfg, p)
	if err != nil {
		RemovePoll(cfg.PollFile)
		return err
	}
	return p.Save()
}

func fillBooks(cfg *config.Config, p *Poll) error {
	cl := notionapi.NewClient(notionapi.Token(cfg.NotionToken))

	v, err := cl.Database.Query(context.Background(), notionapi.DatabaseID(cfg.BookDB), nil)
	if err != nil {
		return err
	}

	for _, v := range v.Results {
		k, ok := v.Properties["Книга"].(*notionapi.TitleProperty)
		if !ok {
			return fmt.Errorf("book cast error")
		}
		var res string
		for _, t := range k.Title {
			res += t.PlainText
		}
		if len(res) == 0 {
			return fmt.Errorf("bad book: %v", k.Title)
		}

		a, ok := v.Properties["Кто предложил"].(*notionapi.PeopleProperty)
		if !ok {
			return fmt.Errorf("author cast error")
		}
		if len(a.People) != 1 {
			return fmt.Errorf("author cast error")
		}
		name := a.People[0].Name
		tg, ok := cfg.NotionTGMap[name]
		if !ok {
			return fmt.Errorf("unknow user: %v", name)
		}
		p.Variants = append(p.Variants, Variant{
			ID:     string(v.ID),
			Text:   res,
			Author: tg,
		})
	}
	return nil
}

func PushResult(cfg *config.Config) error {
	p, err := NewPoll(cfg)
	if err != nil {
		return err
	}
	if !p.Closed {
		return fmt.Errorf("poll is not closed")
	}
	return fillBookResult(cfg, p)
}

func fillBookResult(cfg *config.Config, p *Poll) error {
	cl := notionapi.NewClient(notionapi.Token(cfg.NotionToken))
	shToVariant := make(map[string]Variant, len(p.Variants))
	for _, v := range p.Variants {
		shToVariant[v.Short()] = v
	}
	for uname, state := range p.State {
		for _, vote := range state.Votes {
			_, err := cl.Page.Create(context.Background(), &notionapi.PageCreateRequest{
				Parent: notionapi.Parent{
					Type:       notionapi.ParentTypeDatabaseID,
					DatabaseID: notionapi.DatabaseID(p.ResultDB),
				},
				Properties: map[string]notionapi.Property{
					"Name": notionapi.TitleProperty{
						Type: notionapi.PropertyTypeTitle,
						Title: []notionapi.RichText{
							{
								Type: notionapi.ObjectTypeText,
								Text: notionapi.Text{
									Content: cfg.TGNotionMap[uname],
								},
							},
						},
					},
					"Сколько баллов": notionapi.NumberProperty{
						Type:   notionapi.PropertyTypeNumber,
						Number: float64(vote.Count),
					},
					"Выбор": notionapi.RelationProperty{
						Type: notionapi.PropertyTypeRelation,
						Relation: []notionapi.Relation{
							{
								ID: notionapi.PageID(shToVariant[vote.Short].ID),
							},
						},
					},
				},
			})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
