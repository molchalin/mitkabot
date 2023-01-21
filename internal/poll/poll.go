package poll

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	iuliia "github.com/mehanizm/iuliia-go"
	"github.com/molchalin/mitkabot/internal/config"
	"gopkg.in/yaml.v2"
)

const MaxVotes = 3

const (
	TypeBook   = "book"
	TypeReport = "report"
)

func (s State) MaxPoints() uint {
	if s.Disabled {
		return 0
	}
	if !s.ActivityChecked || s.Activity {
		return 10
	}
	return 7
}

type Poll struct {
	filename string
	Variants []Variant        `yaml:"variants"`
	State    map[string]State `yaml:"state,omitempty"`
	Type     string           `yaml:"type"`
	ResultDB string           `yaml:"result_db"`
	Closed   bool             `yaml:"closed"`

	admins      []string          `yaml:"-"`
	tgNotionMap map[string]string `yaml:"-"`
}

type State struct {
	Disabled        bool   `yaml:"disabled,omitempty"`
	ActivityChecked bool   `yaml:"activity_checked,omitempty"`
	Activity        bool   `yaml:"activity,omitempty"`
	Votes           []Vote `yaml:"votes,omitempty"`
}

type Variant struct {
	Text   string `yaml:"text"`
	Author string `yaml:"author"`
	ID     string `yaml:"id"`
}

func (v Variant) Short() string {
	short := iuliia.Wikipedia.Translate(v.Text)
	words := strings.Split(short, " ")
	w := 3
	if len(words) < 3 {
		w = len(words)
	}
	short = strings.Join(words[:w], " ")

	return string(re.ReplaceAll([]byte(short), []byte("_")))
}

func (v Variant) String() string {
	return fmt.Sprintf("[%s](%s)", v.Text, v.Short())
}

type Vote struct {
	Short string `yaml:"short_name"`
	Count uint   `yaml:"count"`
}

type View struct {
	Text  string
	Short string
	Count uint
	Index uint
}

func (v View) String() string {
	return fmt.Sprintf("%v. %v - **%v**", v.Index, v.Text, v.Count)
}

func pollFile(str string) string {
	return filepath.Join("etc", str+".yml")
}

func CreatePoll(str string) (*Poll, error) {
	f, err := os.OpenFile(pollFile(str), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return nil, err
	}
	f.Close()
	return &Poll{
		filename: pollFile(str),
		State:    make(map[string]State),
	}, nil
}

func RemovePoll(str string) {
	os.Remove(pollFile(str))
}

func NewPoll(cfg *config.Config) (*Poll, error) {
	filename := pollFile(cfg.PollFile)
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	dec := yaml.NewDecoder(f)

	p := &Poll{
		filename:    filename,
		State:       make(map[string]State),
		tgNotionMap: cfg.TGNotionMap,
		admins:      cfg.Admins,
	}
	err = dec.Decode(p)
	if err != nil {
		return nil, err
	}
	if p.Type == "" {
		p.Type = TypeBook
	}
	return p, nil
}

func (p *Poll) Save() error {
	f, err := os.OpenFile(p.filename, os.O_WRONLY|os.O_TRUNC, 0)
	if err != nil {
		return err
	}
	enc := yaml.NewEncoder(f)

	err = enc.Encode(p)

	err2 := enc.Close()
	if err == nil {
		err = err2
	}

	f.Sync()

	err2 = f.Close()
	if err == nil {
		err = err2
	}
	return err
}

func (s State) check() error {
	var sum uint
	for _, v := range s.Votes {
		sum += v.Count
	}
	if sum > s.MaxPoints() {
		return fmt.Errorf("sum overflows %v", s.MaxPoints())
	}
	return nil
}

func (p *Poll) add(old []Vote, vote Vote) []Vote {
	n := make([]Vote, len(old), len(old)+1)
	copy(n, old)
	for i, v := range n {
		if v.Short == vote.Short {
			n[i].Count = vote.Count
			return n
		}
	}
	n = append(n, vote)
	return n
}

func (p *Poll) Vote(name string, vote Vote) error {
	old := p.State[name]
	if len(old.Votes) >= MaxVotes {
		return fmt.Errorf("too many votes")
	}

	n := p.add(old.Votes, vote)
	old.Votes = n
	if err := old.check(); err != nil {
		return err
	}
	p.State[name] = old
	return nil
}

func (p *Poll) del(old []Vote, link string) ([]Vote, bool) {
	n := old[:0]
	var deleted bool
	for _, v := range old {
		if v.Short != link {
			n = append(n, v)
		} else {
			deleted = true
		}
	}
	return n, deleted
}

func (p *Poll) DelVote(name string, sh string) error {
	old := p.State[name]
	n, ok := p.del(old.Votes, sh)
	if !ok {
		return fmt.Errorf("unknown vote")
	}
	old.Votes = n
	p.State[name] = old
	return nil
}

func (p *Poll) getView(name string, notEmpty bool) []View {
	res := make([]View, 0, len(p.Variants))
	cnt := make(map[string]uint)
	for _, v := range p.State[name].Votes {
		cnt[v.Short] = v.Count
	}
	for i, v := range p.Variants {
		if name != v.Author && (!notEmpty || cnt[v.Short()] > 0) {
			res = append(res, View{v.Text, v.Short(), cnt[v.Short()], uint(i + 1)})
		}
	}
	return res
}

func (p *Poll) GetView(name string) []View {
	return p.getView(name, false)
}

func (p *Poll) GetViewNotEmpty(name string) []View {
	return p.getView(name, true)
}

func (p *Poll) Points(name string) (sum uint) {
	for _, v := range p.GetViewNotEmpty(name) {
		sum += v.Count
	}
	return sum
}

func (p *Poll) IsAdmin(name string) bool {
	for _, admin := range p.admins {
		if admin == name {
			return true
		}
	}
	return false
}

func (p *Poll) CanVote(name string) bool {
	return !p.Closed && p.canVote(name)
}

func (p *Poll) canVote(name string) bool {
	return p.Points(name) < p.State[name].MaxPoints() && len(p.GetViewNotEmpty(name)) < MaxVotes
}

func (p *Poll) CanUnvote(name string) bool {
	return !p.Closed && len(p.GetViewNotEmpty(name)) > 0
}

func (p *Poll) CanStop(name string) bool {
	return !p.Closed && p.IsAdmin(name)
}

func (p *Poll) CanResume(name string) bool {
	return p.Closed && p.IsAdmin(name)
}

func (p *Poll) NeedActivityCheck(name string) bool {
	return false && p.Type == TypeBook && !p.State[name].ActivityChecked
}

func (p *Poll) CheckUser(name string) error {
	if _, ok := p.tgNotionMap[name]; !ok {
		return fmt.Errorf("unknown user")
	}
	return nil
}

func (p *Poll) Progress() ([]string, int) {
	var noVote []string
	cnt := len(p.tgNotionMap)
	for name := range p.tgNotionMap {
		if p.canVote(name) {
			noVote = append(noVote, name)
		}
		if p.State[name].Disabled {
			cnt--
		}
	}
	return noVote, cnt
}

func (p *Poll) Result(empty bool) []View {
	res := make([]View, 0, len(p.Variants))
	cnt := make(map[string]uint)
	for _, state := range p.State {
		for _, vote := range state.Votes {
			cnt[vote.Short] += vote.Count
		}
	}
	for i, v := range p.Variants {
		if empty || cnt[v.Short()] > 0 {
			res = append(res, View{v.Text, v.Short(), cnt[v.Short()], uint(i + 1)})
		}
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].Count >= res[j].Count
	})
	return res
}
