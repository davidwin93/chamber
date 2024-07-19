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
	"sync/atomic"
	"time"

	"github.com/sethvargo/go-retry"
)

type VMConfig interface {
	GetIP() string
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
	srcPort               string
	dstPort               string
	dstIP                 string
	vm                    VMConfig
	machine               *vm.ActiveVM
	lastObservedTimestamp int64
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
		conn.SetDeadline(time.Now().Add(10 * time.Second))
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
		machine := vm.NewVM(&vm.VMDefinition{IP: t.vm.GetIP(), ID: t.vm.GetID(), KernelPath: t.vm.GetKernelImage(), RootDrive: t.vm.GetRootDrive()}, config.LoadDefaultConfig())
		err = machine.StartNewMachine(context.Background())
		if err != nil {
			fmt.Print(err)
			return
		}
		fmt.Println("VM created")
		t.machine = machine
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
	progressCon := NewProgressWrapper(conn, &t.lastObservedTimestamp)
	progressOut := NewProgressWrapper(outerConn, &t.lastObservedTimestamp)
	go func() {
		defer wg.Done()
		io.Copy(outerConn, progressCon)
	}()
	go func() {
		defer wg.Done()
		io.Copy(conn, progressOut)
	}()
	wg.Wait()
	log.Println("Connection closed. Starting snapshot")
	// need to hanlde multiple connections to VM since 1 terminating shouldn't cause the VM to be stopped
	err = t.machine.ShutdownAndSnapshot(fmt.Sprintf("./mem-snapshot-%s.snapshot", t.vm.GetID()), fmt.Sprintf("./config-%s.snapshot", t.vm.GetID()))
	if err != nil {
		log.Println("Error in snapshotting", err)

	}
}

type ProgressWrapper struct {
	addr *int64
	r    io.Reader
}

func NewProgressWrapper(reader io.Reader, addr *int64) *ProgressWrapper {
	return &ProgressWrapper{
		addr: addr,
		r:    reader,
	}
}
func (p *ProgressWrapper) Read(b []byte) (n int, err error) {
	n, err = p.r.Read(b)
	atomic.StoreInt64(p.addr, time.Now().Unix())
	return
}
func (p *ProgressWrapper) GetIdleTime() time.Duration {
	return time.Since(time.Unix(atomic.LoadInt64(p.addr), 0))
}
