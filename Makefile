FLAGS = GOOS=linux GOARCH=amd64

build: mitka mitkactl notify

mitka:
	$(FLAGS) go build -o bin/mitka github.com/molchalin/mitkabot/cmd/mitka

mitkactl:
	$(FLAGS) go build -o bin/mitkactl github.com/molchalin/mitkabot/cmd/mitkactl

notify:
	$(FLAGS) go build -o bin/notify github.com/molchalin/mitkabot/cmd/notify

bot:
	bin/mitka

mk: 
	bin/mitkactl --tool mk

mk_rep:
	bin/mitkactl --tool mk_rep

push:
	bin/mitkactl --tool push

update:
	scp mitka.yml root@51.83.170.104:/etc/mitka.yml
	scp bin/mitka root@51.83.170.104:/root/bin/mitka
	scp bin/mitkactl root@51.83.170.104:/root/bin/mitkactl
	scp bin/notify root@51.83.170.104:/root/bin/notify
	scp Makefile root@51.83.170.104:/root/Makefile

clean:
	rm -rf bin
