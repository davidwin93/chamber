package main

import (
	"chamber/internal/images"
	"chamber/internal/server"
	"log"
	"sync"

	"github.com/google/uuid"
)

type VMData struct {
	IP    byte
	Drive string
}
type PortMap struct {
	IncomingPort    string
	DestinationPort string
	DestinationIP   string
	Protocol        string
	VM              *VMData
}

const BUFFER_SIZE = 1024 * 64

func main() {
	log.Print(images.PullImage("hello-world"))

	// Listen for incoming connections
	alpineNginx := &VMData{
		IP:    4,
		Drive: "./vm/alpine.ext4",
	}
	alpineSSH := &VMData{
		IP:    5,
		Drive: "./output.ext4",
	}
	portMappings := []*PortMap{
		&PortMap{
			IncomingPort:    "8091",
			DestinationPort: "22",
			DestinationIP:   "172.102.0.5",
			Protocol:        "tcp",
			VM:              alpineSSH,
		},
		&PortMap{
			IncomingPort:    "8094",
			DestinationPort: "80",
			DestinationIP:   "172.102.0.5",
			Protocol:        "tcp",
			VM:              alpineSSH,
		},
		&PortMap{
			IncomingPort:    "8092",
			DestinationPort: "80",
			DestinationIP:   "172.102.0.4",
			Protocol:        "tcp",
			VM:              alpineNginx,
		},
		&PortMap{
			IncomingPort:    "8093",
			DestinationPort: "22",
			DestinationIP:   "172.102.0.4",
			Protocol:        "tcp",
			VM:              alpineNginx,
		},
	}
	var wg sync.WaitGroup
	for _, m := range portMappings {
		wg.Add(1)
		go func(m *PortMap) {
			defer wg.Done()
			srv := server.NewTCPServer(&AlpineVM{
				id:     uuid.NewString(),
				ip:     m.VM.IP,
				drive:  m.VM.Drive,
				kernel: "./vm/vmlinux-6.1.0.bin",
			}, m.IncomingPort, m.DestinationPort, m.DestinationIP)
			log.Println(srv.Start())
		}(m)
	}
	wg.Wait()
}

type AlpineVM struct {
	id     string
	ip     byte
	kernel string
	drive  string
}

func (a *AlpineVM) GetIPByte() byte {
	return a.ip
}
func (a *AlpineVM) GetKernelImage() string {
	return a.kernel
}
func (a *AlpineVM) GetRootDrive() string {
	return a.drive
}
func (a *AlpineVM) GetID() string {
	return a.id
}
