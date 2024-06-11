package vm

import (
	"chamber/internal/config"
	"context"
	secure "crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/firecracker-microvm/firecracker-go-sdk"
	"github.com/firecracker-microvm/firecracker-go-sdk/client/models"
)

type VMConfig struct {
	snapshotLocation string
	ip               string
}

type Options struct {
	Id string `long:"id" description:"Jailer VMM id"`
	// maybe make this an int instead
	IP              string `sdtring:"ip" description:"an ip we use to generate an ip address"`
	FcBinary        string `long:"firecracker-binary" description:"Path to firecracker binary"`
	FcKernelCmdLine string `long:"kernel-opts" description:"Kernel commandline"`
	RootDrivePath   string `long:"root-drive-path" description:"Root Drive Path"`
	KernelPath      string `long:"kernel-path" description:"Kernel Path"`
	FcSocketPath    string `long:"socket-path" short:"s" description:"path to use for firecracker socket"`
	TapMacAddr      string `long:"tap-mac-addr" description:"tap macaddress"`
	TapDev          string `long:"tap-dev" description:"tap device"`
	FcCPUCount      int64  `long:"ncpus" short:"c" description:"Number of CPUs"`
	FcMemSz         int64  `long:"memory" short:"m" description:"VM memory, in MiB"`
	FcIP            string `long:"fc-ip" description:"IP address of the VM"`
}

type ActiveVM struct {
	ctx       context.Context
	cancelCtx context.CancelFunc
	image     string
	machine   *firecracker.Machine
	id        string
	config    *VMConfig
	opts      *Options
}
type snapshotOpt struct {
	snapshot firecracker.Opt
}

type VMDefinition struct {
	RootDrive  string
	KernelPath string
	ID         string
	IP         string
}

const (
	local     = 0b10
	multicast = 0b1
)

func generateMacAddress() string {
	buf := make([]byte, 6)
	_, err := secure.Read(buf)
	if err != nil {
		fmt.Println("error:", err)
		return ""
	}
	// clear multicast bit (&^), ensure local bit (|)
	buf[0] = buf[0]&^multicast | local
	return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", buf[0], buf[1], buf[2], buf[3], buf[4], buf[5])
}

func NewVM(def *VMDefinition, config *config.Config) *ActiveVM {
	//removed ro
	bootArgs := "console=ttyS0 noapic reboot=k panic=1 pci=off nomodules random.trust_cpu=on i8042.noaux i8042.nomux i8042.nopnp i8042.nokbd init=/init-runner"
	macAddress := generateMacAddress()
	if macAddress == "" {
		log.Fatal("could not create mac address")
	}
	unqiueID := rand.Intn(99999-10+1) + 10
	return &ActiveVM{
		opts: &Options{
			RootDrivePath:   def.RootDrive,
			KernelPath:      def.KernelPath,
			IP:              def.IP,
			Id:              def.ID,
			FcBinary:        config.FirecrackerBinaryPath,
			FcKernelCmdLine: bootArgs,
			FcSocketPath:    fmt.Sprintf("/tmp/firecracker-%d.sock", unqiueID),
			TapMacAddr:      macAddress,
			TapDev:          fmt.Sprintf("fc-%d", unqiueID),
			FcIP:            def.IP,
			FcCPUCount:      1,
			FcMemSz:         512,
		},
		id: def.ID,
	}
}
func (vm *ActiveVM) LoadSnapshot(ctx context.Context) error {
	_, err := os.Stat(filepath.Join(vm.config.snapshotLocation, "mem.bin"))
	if err != nil {
		return err
	}
	return vm.createMachine(ctx, &snapshotOpt{firecracker.WithSnapshot("./snapshot/memory.bin", "./snapshot/state.bin", func(sc *firecracker.SnapshotConfig) { sc.ResumeVM = true })})
}
func (vm *ActiveVM) StartNewMachine(ctx context.Context) error {
	return vm.createMachine(ctx, nil)
}
func (vm *ActiveVM) createMachine(ctx context.Context, snapshotOpt *snapshotOpt) error {

	vmmCtx, vmmCancel := context.WithCancel(ctx)
	vm.cancelCtx = vmmCancel
	//rootImagePath, err := copyImage(vm.opts.RootDrivePath)
	//vm.opts.RootDrivePath = rootImagePath
	// if err != nil {
	// 	return fmt.Errorf("Failed copying root path: %s", err)
	// }
	fcCfg, err := vm.opts.getConfig()
	//fcCfg, err := opts.getConfig(firecracker.SnapshotConfig{})
	if err != nil {
		log.Println("Got error", err)
		return err
	}

	cmd := firecracker.VMCommandBuilder{}.
		WithBin(vm.opts.FcBinary).
		WithSocketPath(fcCfg.SocketPath).
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		Build(ctx)

	machineOpts := []firecracker.Opt{
		firecracker.WithProcessRunner(cmd),
	}
	if snapshotOpt != nil {
		machineOpts = append(machineOpts, snapshotOpt.snapshot)
	}

	err = vm.configureNetwork()
	if err != nil {
		return fmt.Errorf("Failed configuring network: %s", err)
	}

	m, err := firecracker.NewMachine(vmmCtx, *fcCfg, machineOpts...)
	if err != nil {
		return fmt.Errorf("Failed creating machine: %s", err)
	}

	if err := m.Start(vmmCtx); err != nil {
		return fmt.Errorf("Failed to start machine: %v", err)
	}

	inf, err := m.DescribeInstanceInfo(vmmCtx)
	fmt.Printf("%+v", *inf.State)
	return nil
}
func (vm *ActiveVM) configureNetwork() error {
	exec.Command("ip", "link", "del", vm.opts.TapDev).Run()
	if err := exec.Command("ip", "tuntap", "add", "dev", vm.opts.TapDev, "mode", "tap").Run(); err != nil {
		return fmt.Errorf("Failed creating ip link: %s", err)
	}
	if err := exec.Command("rm", "-f", vm.opts.FcSocketPath).Run(); err != nil {
		return fmt.Errorf("Failed to delete old socket path: %s", err)
	}
	if err := exec.Command("ip", "link", "set", vm.opts.TapDev, "master", "firecracker0").Run(); err != nil {
		return fmt.Errorf("Failed adding tap device to bridge: %s", err)
	}
	if err := exec.Command("ip", "link", "set", vm.opts.TapDev, "up").Run(); err != nil {
		return fmt.Errorf("Failed creating ip link: %s", err)
	}
	if err := exec.Command("sysctl", "-w", fmt.Sprintf("net.ipv4.conf.%s.proxy_arp=1", vm.opts.TapDev)).Run(); err != nil {
		return fmt.Errorf("Failed doing first sysctl: %s", err)
	}
	if err := exec.Command("sysctl", "-w", fmt.Sprintf("net.ipv6.conf.%s.disable_ipv6=1", vm.opts.TapDev)).Run(); err != nil {
		return fmt.Errorf("Failed doing second sysctl: %s", err)
	}
	return nil
}
func (opts *Options) getConfig() (*firecracker.Config, error) {
	drives := []models.Drive{
		models.Drive{
			DriveID:      firecracker.String("1"),
			PathOnHost:   &opts.RootDrivePath,
			IsRootDevice: firecracker.Bool(true),
			IsReadOnly:   firecracker.Bool(false),
		},
	}
	fc_ip := net.ParseIP(opts.IP)
	//gateway_ip := "172.102.0.1"
	docker_mask_long := config.GetConfig().SubnetMask
	return &firecracker.Config{
		VMID:            opts.Id,
		SocketPath:      opts.FcSocketPath,
		KernelImagePath: opts.KernelPath,
		KernelArgs:      opts.FcKernelCmdLine,
		Drives:          drives,
		NetworkInterfaces: []firecracker.NetworkInterface{
			firecracker.NetworkInterface{
				StaticConfiguration: &firecracker.StaticNetworkConfiguration{
					MacAddress:  opts.TapMacAddr,
					HostDevName: opts.TapDev,
					IPConfiguration: &firecracker.IPConfiguration{
						IPAddr:  net.IPNet{IP: fc_ip, Mask: net.IPMask(docker_mask_long)},
						Gateway: config.GetConfig().GatewayIP,
						IfName:  "eth0",
					},
				},
				//AllowMMDS: allowMMDS,
			},
		},
		MachineCfg: models.MachineConfiguration{
			VcpuCount:  firecracker.Int64(opts.FcCPUCount),
			MemSizeMib: firecracker.Int64(opts.FcMemSz),
			//CPUTemplate: models.CPUTemplate(opts.FcCPUTemplate),
			Smt: firecracker.Bool(false),
		},
		//JailerCfg: jail,
		//VsockDevices:      vsocks,
		//LogFifo:           opts.FcLogFifo,
		//LogLevel:          opts.FcLogLevel,
		//MetricsFifo:       opts.FcMetricsFifo,
		//FifoLogWriter:     fifo,
		//Snapshot: snapshot,
	}, nil
}

func copyImage(src string) (string, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return "", err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer source.Close()

	destination, err := ioutil.TempFile("/tmp/images", "image")
	if err != nil {
		return "", err
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	return destination.Name(), err
}
