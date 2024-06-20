package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
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
	err = exec.Command("/bin/mount", "-t", "proc", "proc", "/proc").Run()
	if err != nil {
		log.Fatal("could not mount proc", err)
	}
	err = exec.Command("/bin/mount", "-t", "sysfs", "none", "/sys").Run()
	if err != nil {
		log.Fatal("could not mount sys", err)
	}
	err = exec.Command("/bin/mkdir", "-p", "/dev/pts").Run()
	if err != nil {
		log.Fatal("could not create dev/pts", err)
	}
	devMnt := exec.Command("/bin/mount", "-t", "devpts", "devpts", "/dev/pts")
	devMnt.Stderr = os.Stderr
	devMnt.Stdout = os.Stdout
	devMnt.Stdin = nullReader{}
	err = devMnt.Run()
	if err != nil {
		log.Fatal("could not mount dev", err)
	}
	linkFD := exec.Command("/bin/ln", "-s", "/proc/self/fd", "/dev/fd")
	linkFD.Stderr = os.Stderr
	linkFD.Stdout = os.Stdout
	linkFD.Stdin = nullReader{}
	err = linkFD.Run()
	if err != nil {
		log.Fatal("could not mount dev", err)
	}

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
