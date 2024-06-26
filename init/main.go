package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

type nullReader struct{}
type nullWriter struct{}

func (nullReader) Read(p []byte) (n int, err error)  { return len(p), nil }
func (nullWriter) Write(p []byte) (n int, err error) { return len(p), nil }

func main() {
	// write hello world to /test.txt
	test, err := os.Create("/test.txt")
	if err != nil {

		log.Fatal(err)
	}
	test.WriteString("Hello, World!")
	test.Close()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	err = prepEnv()
	if err != nil {
		log.Println(err)
	}
	// exec.Command("/bin/mkdir", "-p", "/dev/null").Run()
	// err = exec.Command("/bin/mknod", "-m", "0666", "/dev/null", "c", "1", "3").Run()
	// if err != nil {
	// 	log.Fatal("could not create /dev/null", err)
	// }
	go func() {

		file, err := os.Open("/command.json")
		if err != nil {
			log.Fatal(err)
		}
		defer file.Close()
		var config map[string][]string
		err = json.NewDecoder(file).Decode(&config)
		if err != nil {
			log.Fatal(err)
		}
		if len(config["command"]) == 0 {
			log.Println("command not found")
			return
		}
		env := config["env"]
		cmdPath := config["command"][0]
		if !strings.HasPrefix(cmdPath, "/") && !strings.HasPrefix(cmdPath, "./") {
			cmdPath = "./" + cmdPath
		}
		if len(config["command"]) == 1 {
			cmd := exec.Command(cmdPath)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = nullReader{}
			cmd.Env = env
			err = cmd.Run()
			if err != nil {
				log.Fatal(err)
			}
		} else {
			cmd := exec.Command(cmdPath, config["command"][1:]...)

			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = nullReader{}
			cmd.Env = env

			err = cmd.Run()
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	log.Fatal(http.ListenAndServe(":2222", nil))
}
func prepEnv() error {

	os.MkdirAll("/dev/pts", 0755)
	os.MkdirAll("/proc", 0755)
	os.MkdirAll("/sys", 0755)
	err := unix.Mknod("/dev/null", 1, 3)
	if err != nil {
		log.Println("could not crate null %w", err)
	}
	unix.Mount("devtmpfs", "/dev", "devtmpfs", 0, "")
	//exec.Command("/bin/mount", "-t", "devtmpfs", "devtmpfs", "/dev").Run()
	err = unix.Mount("proc", "/proc", "proc", 0, "")
	// err = exec.Command("/bin/mount", "-t", "proc", "proc", "/proc").Run()
	if err != nil {
		return fmt.Errorf("could not mount proc %w", err)
	}
	err = unix.Mount("sysfs", "/sys", "sysfs", 0, "")
	if err != nil {
		return fmt.Errorf("could not mount sys %w", err)
	}

	err = unix.Mount("devpts", "/dev/pts", "devpts", 0, "")
	if err != nil {
		return fmt.Errorf("could not mount devpts %w", err)
	}

	err = unix.Symlink("/proc/self/fd", "/dev/fd")
	if err != nil {
		return fmt.Errorf("could not mount dev %w", err)
	}
	return nil
}
