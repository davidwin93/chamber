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

.PHONY: network
network:
	sudo ip link add name firecracker0 type bridge && sudo ip addr add 172.102.0.1/16 dev firecracker0 && sudo ip link set dev firecracker0 up

.PHONY: test-instances
test-instances:
	curl --header "Content-Type: application/json" --request POST --data '{"name":"go-httpbin","image":"mccutchen/go-httpbin","dstPort":"8080","protocol":"tcp"}' http://localhost:8070/vm