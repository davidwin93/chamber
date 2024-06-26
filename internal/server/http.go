package server

import (
	"chamber/internal/images"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"

	"github.com/google/uuid"
)

type APIServer struct {
	assignedIPs    map[string]struct{}
	allocatedPorts map[int]struct{}
	portLock       sync.Mutex
	ipLock         sync.Mutex
	activeServers  map[string]*ActiveServer
	serverLock     sync.Mutex
}

type Workload struct {
	Name            string   `json:"name"`
	Image           string   `json:"image"`
	DestinationPort string   `json:"dstPort"`
	Protocol        string   `json:"protocol"`
	Env             []string `json:"env"`
}

type GenericVM struct {
	id     string
	ip     string
	kernel string
	drive  string
}
type ActiveServer struct {
	Server *TCPServer
}

func (a *GenericVM) GetIP() string {
	return a.ip
}
func (a *GenericVM) GetKernelImage() string {
	return a.kernel
}
func (a *GenericVM) GetRootDrive() string {
	return a.drive
}
func (a *GenericVM) GetID() string {
	return a.id
}

func (api *APIServer) StartAPIServer(port string) error {
	api.assignedIPs = make(map[string]struct{})
	api.allocatedPorts = make(map[int]struct{})
	api.activeServers = make(map[string]*ActiveServer)

	http.HandleFunc("/vm", api.createVM)
	log.Println("Starting API server on port", port)
	return http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
}
func (api *APIServer) createVM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var workload Workload
	err := json.NewDecoder(r.Body).Decode(&workload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Println(err)
		return
	}
	log.Println("Creating VM for", workload.Name)
	ip := api.getUnusedIP()
	port := api.getUnusedPort()
	log.Println("Using IP", ip, "and port", port)
	rootDrive, err := images.PullImage(workload.Image, workload.Env)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println(rootDrive, "has been created for", workload.Image)
	id := uuid.NewString()

	srv := NewTCPServer(&GenericVM{
		id:     id,
		ip:     ip,
		drive:  rootDrive,
		kernel: "./vm/vmlinux-6.1.0.bin",
	}, port, workload.DestinationPort, ip)

	api.serverLock.Lock()
	defer api.serverLock.Unlock()
	api.activeServers[id] = &ActiveServer{srv}
	go func() {
		log.Println(srv.Start())
	}()
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(&VMStatus{ID: id, Port: port, Name: workload.Name})
	if err != nil {
		log.Println(err)
	}
}

type VMStatus struct {
	ID   string `json:"id"`
	Port string `json:"port"`
	Name string `json:"name"`
}

func (api *APIServer) getUnusedIP() string {
	api.ipLock.Lock()
	defer api.ipLock.Unlock()
	for {
		lastByte := rand.Intn(254-1+1) + 1
		midByte := rand.Intn(254-1+1) + 1
		ip := fmt.Sprintf("172.102.%d.%d", midByte, lastByte)
		if _, ok := api.assignedIPs[ip]; ok {
			continue
		}
		api.assignedIPs[ip] = struct{}{}
		return ip
	}
}

func (api *APIServer) getUnusedPort() string {
	api.portLock.Lock()
	defer api.portLock.Unlock()
	for {
		port := rand.Intn(64000-1000+1) + 1000

		if _, ok := api.allocatedPorts[port]; ok {
			continue
		}
		api.allocatedPorts[port] = struct{}{}
		return fmt.Sprintf("%d", port)
	}
}
