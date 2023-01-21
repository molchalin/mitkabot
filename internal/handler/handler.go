package handler

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/molchalin/mitkabot/internal/poll"
)

var globalAdmins = []string{"molchalin"}

func IsGlobalAdmin(name string) bool {
	for _, adm := range globalAdmins {
		if adm == name {
			return true
		}
	}
	return false
}

type userState int

const (
	userStateCmd = userState(iota)
	userStateVoteSelect
	userStateVotePoints
	userStateUnvoteSelect
	userStateActivityCheck
)

func (s userState) String() string {
	switch s {
	case userStateCmd:
		return "cmd"
	case userStateVoteSelect:
		return "vote_select"
	case userStateVotePoints:
		return "vote_points"
	case userStateUnvoteSelect:
		return "unvote_select"
	}
	panic(fmt.Sprintf("unknown userState: %d", s))
}

type Dispatcher struct {
	p      *poll.Poll
	m      map[string]Handler
	state  map[string]userState
	choice map[string]string
}

type Handler struct {
	Path     string
	F        func(string, []string) error
	Argc     uint
	State    userState
	AnyState bool
}

func (d *Dispatcher) exec(h Handler, uname string, args []string) error {
	if err := d.p.CheckUser(uname); err != nil {
		return err
	}
	if uint(len(args)) != h.Argc {
		return fmt.Errorf("bad argc for cmd=%v: got=%v, want=%v", h.Path, len(args), h.Argc)
	}
	if !h.AnyState && d.state[uname] != h.State {
		return fmt.Errorf("bad state for cmd=%v: got=%v, want=%v", h.Path, d.state[uname], h.State)
	}
	return h.F(uname, args)
}

func (d *Dispatcher) Handler(uname string, argStr string) {
	args := strings.Split(argStr, " ")
	h, ok := d.m[args[0]]
	if !ok {
		log.Printf("WARN: unknown command %v %v", args[0], argStr)
		return
	}
	err := d.exec(h, uname, args[1:])
	if err != nil {
		log.Printf("WARN: user=%v, argStr=%v err: %v", uname, argStr, err)
	}
}

func NewDispatcher(p *poll.Poll) *Dispatcher {
	d := &Dispatcher{
		p:      p,
		state:  make(map[string]userState),
		choice: make(map[string]string),
		m:      make(map[string]Handler),
	}

	for _, h := range []Handler{
		{
			Path: "update",
			F:    d.update,
		},
		{
			Path: "vote",
			F:    d.vote,
		},
		{
			Path:  "vote_sel",
			F:     d.voteSel,
			Argc:  1,
			State: userStateVoteSelect,
		},
		{
			Path: "unvote",
			F:    d.unvote,
		},
		{
			Path:  "unvote_sel",
			F:     d.unvoteSel,
			Argc:  1,
			State: userStateUnvoteSelect,
		},
		{
			Path:  "vote_cnt",
			F:     d.voteCnt,
			Argc:  1,
			State: userStateVotePoints,
		},
		{
			Path: "stop",
			F:    d.stop,
		},
		{
			Path: "resume",
			F:    d.resume,
		},
		{
			Path:     "menu",
			F:        d.menu,
			AnyState: true,
		},
		{
			Path:  "activity",
			F:     d.activity,
			Argc:  1,
			State: userStateActivityCheck,
		},
	} {
		d.m[h.Path] = h
	}
	return d
}

func checkArgc(args []string, cnt int) error {
	return nil
}

var ErrParse = fmt.Errorf("cant parse arg")

func (d *Dispatcher) vote(name string, args []string) error {
	if d.p.CanVote(name) {
		if d.p.NeedActivityCheck(name) {
			d.state[name] = userStateActivityCheck
		} else {
			d.state[name] = userStateVoteSelect
		}
	}
	return nil
}

func (d *Dispatcher) voteSel(name string, args []string) error {
	if d.p.CanVote(name) {
		d.state[name] = userStateVotePoints
		d.choice[name] = args[0]
	} else {
		d.state[name] = userStateCmd
	}
	return nil
}

func (d *Dispatcher) unvote(name string, args []string) error {
	if d.state[name] != userStateCmd {
		return fmt.Errorf("bad state")
	}
	if d.p.CanUnvote(name) {
		d.state[name] = userStateUnvoteSelect
	}
	return nil
}

func (d *Dispatcher) unvoteSel(name string, args []string) error {
	d.state[name] = userStateCmd
	if !d.p.CanUnvote(name) {
		return nil
	}
	err := d.p.DelVote(name, args[0])
	if err != nil {
		return err
	}
	return d.p.Save()
}

func (d *Dispatcher) voteCnt(name string, args []string) error {
	cnt, err := strconv.ParseUint(args[0], 10, 64)
	if err != nil {
		return err
	}
	d.state[name] = userStateCmd
	if !d.p.CanVote(name) {
		return nil
	}
	choice := d.choice[name]
	delete(d.choice, name)
	err = d.p.Vote(name, poll.Vote{choice, uint(cnt)})
	if err != nil {
		return err
	}
	return d.p.Save()
}

func (d *Dispatcher) update(name string, args []string) error {
	return nil
}

func (d *Dispatcher) stop(name string, args []string) error {
	if !d.p.CanStop(name) {
		return fmt.Errorf("you cant stop poll")
	}
	d.p.Closed = true
	return d.p.Save()
}

func (d *Dispatcher) resume(name string, args []string) error {
	if !d.p.CanResume(name) {
		return fmt.Errorf("you cant resume poll")
	}
	d.p.Closed = false
	return d.p.Save()
}

func (d *Dispatcher) menu(name string, args []string) error {
	d.state[name] = userStateCmd
	return nil
}

func (d *Dispatcher) activity(name string, args []string) error {
	if !d.p.NeedActivityCheck(name) {
		return fmt.Errorf("no need for activity check")
	}
	s := d.p.State[name]
	s.ActivityChecked = true
	s.Activity = args[0] == "true"
	d.p.State[name] = s
	d.p.Save()
	d.state[name] = userStateVoteSelect
	return nil
}

func viewsToButtons(vs []poll.View, cmd string) (res [][]tgbotapi.InlineKeyboardButton) {
	for _, v := range vs {
		res = append(res,
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData(
					fmt.Sprintf("%v. %v", v.Index, v.Text), fmt.Sprintf("%v %v", cmd, v.Short),
				)))
	}
	return res
}

func (d *Dispatcher) Buttons(name string) (res [][]tgbotapi.InlineKeyboardButton) {
	if d.p.CheckUser(name) != nil {
		return nil
	}
	switch d.state[name] {
	case userStateCmd:
		res = append(res, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Обновить", "update")))
		if d.p.CanVote(name) {
			res = append(res, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Проголосовать", "vote")))
		}
		if d.p.CanUnvote(name) {
			res = append(res, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Удалить голос", "unvote")))
		}
		if d.p.CanStop(name) {
			res = append(res, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Остановить голосование", "stop")))
		}
		if d.p.CanResume(name) {
			res = append(res, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Возобновить голосование", "resume")))
		}
		if IsGlobalAdmin(name) {
			res = append(res, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Сменить голосование", "change")))
		}
		return res
	case userStateVoteSelect:
		res = viewsToButtons(d.p.GetView(name), "vote_sel")
	case userStateUnvoteSelect:
		res = viewsToButtons(d.p.GetViewNotEmpty(name), "unvote_sel")
	case userStateVotePoints:
		for i := uint(1); i <= d.p.State[name].MaxPoints()-d.p.Points(name); i++ {
			res = append(res, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(strconv.Itoa(int(i)), fmt.Sprintf("vote_cnt %v", i))))
		}
	case userStateActivityCheck:
		res = append(res,
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Да", "activity true")),
			tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Нет", "activity false")),
		)
	}
	res = append(res, tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData("Вернуться в меню", "menu")))
	return res
}

func mapString(arr []string, f func(s string) string) []string {
	res := make([]string, len(arr))
	for i, el := range arr {
		res[i] = f(el)
	}
	return res
}

func mention(name string) string {
	return fmt.Sprintf("@%v", name)
}

func (d *Dispatcher) userNotFound(b *strings.Builder, name string) {
	b.WriteString("Не заполнена информация о вашем Notion.")
}

func convStr(arr interface{}) []string {
	v := reflect.ValueOf(arr)
	if v.Kind() != reflect.Slice {
		panic("slice expected")
	}
	res := make([]string, v.Len())
	for i := 0; i < v.Len(); i++ {
		res[i] = v.Index(i).Interface().(fmt.Stringer).String()
	}
	return res
}

func (d *Dispatcher) yourChoice(b *strings.Builder, name string) {
	votes := d.p.GetViewNotEmpty(name)
	if d.p.Closed {
		b.WriteString("Голосование окончено!\n")
	} else if len(votes) == 0 {
		b.WriteString("Вы еще не проголосовали\n")
	}

	if len(votes) == 0 {
		return
	}
	b.WriteString("Ваш выбор:\n")
	b.WriteString(strings.Join(convStr(votes), "\n"))
	b.WriteString("\n")
	return
}

func (d *Dispatcher) progress(b *strings.Builder, name string) {
	b.WriteString("\n")
	noVote, total := d.p.Progress()
	b.WriteString(fmt.Sprintf("Проголосовало: %d/%d\n", total-len(noVote), total))

	if !d.p.IsAdmin(name) && !d.p.Closed {
		return
	}
	votes := d.p.Result(false)
	b.WriteString("\n")
	b.WriteString("Результат:\n")
	b.WriteString(strings.Join(convStr(votes), "\n"))
	b.WriteString("\n")
}

func (d *Dispatcher) Text(name string) string {
	b := new(strings.Builder)

	if d.p.CheckUser(name) != nil {
		d.userNotFound(b, name)
		return b.String()
	}
	switch d.state[name] {
	case userStateCmd:
		d.yourChoice(b, name)
		d.progress(b, name)
	case userStateVoteSelect, userStateUnvoteSelect:
		t := "книгу"
		if d.p.Type == poll.TypeReport {
			t = "рецензию"
		}
		b.WriteString(fmt.Sprintf("Выберите %v", t))
	case userStateVotePoints:
		b.WriteString("Выберите количество баллов")
	case userStateActivityCheck:
		b.WriteString("В прошлом месяце вы читали книгу, учавствовали в обсуждении и т.д.(Новым участникам жать да) ?")
	default:
		log.Fatalf("cant figure text for userState: %v", d.state[name])
	}
	return b.String()
}
