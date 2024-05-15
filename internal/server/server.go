package server

import (
	"chamber/internal/config"
	"chamber/internal/vm"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/sethvargo/go-retry"
)

type VMConfig interface {
	GetIPByte() byte
	GetKernelImage() string
	GetRootDrive() string
	GetID() string
}
type ProxyServer interface {
	GetListeningPort() string
	GetDestinationPort() string
	GetDestinationIP() string
	Start() error
}
type TCPServer struct {
	srcPort string
	dstPort string
	dstIP   string
	vm      VMConfig
}

func NewTCPServer(vmConfig VMConfig, srcPort, dstPort, dstIP string) *TCPServer {
	return &TCPServer{
		srcPort: srcPort,
		dstPort: dstPort,
		dstIP:   dstIP,
		vm:      vmConfig,
	}
}
func (t *TCPServer) GetListeningPort() string {
	return t.srcPort
}

func (t *TCPServer) GetDestinationIP() string {
	return t.dstIP
}

func (t *TCPServer) GetDestinationPort() string {
	return t.dstPort
}
func (t *TCPServer) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", t.srcPort))
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		// Accept incoming connections
		conn, err := listener.Accept()
		if err != nil {
			continue
		}

		// Handle client connection in a goroutine
		go t.handleClient(conn)
	}
}
func (t *TCPServer) handleClient(conn net.Conn) {
	defer conn.Close()
	timeout := time.Second
	outerConn, err := net.DialTimeout("tcp", net.JoinHostPort(t.GetDestinationIP(), t.GetDestinationPort()), timeout)
	if err != nil {
		fmt.Println("Connecting error:", err)
		vm := vm.NewVM(&vm.VMDefinition{IPByte: t.vm.GetIPByte(), ID: t.vm.GetID(), KernelPath: t.vm.GetKernelImage(), RootDrive: t.vm.GetRootDrive()}, config.LoadDefaultConfig())
		err = vm.StartNewMachine(context.Background())
		if err != nil {
			fmt.Print(err)
			return
		}
		fmt.Println("VM created")
		b := retry.NewConstant(500 * time.Millisecond)
		b = retry.WithMaxRetries(10, b)
		err = retry.Do(context.Background(), b, func(ctx context.Context) error {
			outerConn, err = net.DialTimeout("tcp", net.JoinHostPort(t.GetDestinationIP(), t.GetDestinationPort()), timeout)
			if err != nil {
				return retry.RetryableError(err)
			}
			return nil
		})
		if err != nil {
			log.Println("Server not up", err)
			return
		}

	}
	//else if vm != nil {
	// 	log.Println("Snapshotting VM")
	// 	//vm.Snapshot("./snapshot")
	// }
	defer outerConn.Close()
	log.Println("Connection established")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		io.Copy(outerConn, conn)
	}()
	go func() {
		defer wg.Done()
		io.Copy(conn, outerConn)
	}()
	wg.Wait()
	log.Println("Connection closed")
}
