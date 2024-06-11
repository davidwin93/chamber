package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
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
		if len(config["command"]) == 1 {
			cmd := exec.Command(config["command"][0])
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = nullReader{}
			err = cmd.Run()
			if err != nil {
				log.Fatal(err)
			}
		} else {
			cmd := exec.Command(config["command"][0], config["command"][1:]...)

			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Stdin = nullReader{}
			err = cmd.Run()
			if err != nil {
				log.Fatal(err)
			}
		}
	}()
	log.Fatal(http.ListenAndServe(":2222", nil))
}
