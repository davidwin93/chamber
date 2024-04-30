.PHONY: clean
clean:
	rm -rf /tmp/images/*
.PHONY: build
build:
	go build -v -o runner
.PHONY: run
run: build
	sudo ./runner
.PHONY: e2e
e2e: build
	cd init && CGO_ENABLED=0 go build -v -o init && mv init ../vm && cd ../vm && sudo ./alpine-builder.sh Dockerfile.alpine alpine && rm init

.PHONY: all
all: e2e run