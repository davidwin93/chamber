.PHONY: clean
clean:
	rm -rf /tmp/images/*
.PHONY: build
build:
	go build -v -o runner && rm -rf output* && rm -rf *.tar && rm -rf *.ext4
.PHONY: run
run: build
	sudo dmsetup remove_all && sudo ./runner
.PHONY: e2e
e2e: build
	cd init && CGO_ENABLED=0 go build -v -o init-runner && mv init-runner ../init-runner

.PHONY: all
all: e2e run