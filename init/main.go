package main

import (
	"os"
	"os/exec"
)

func main() {
	// write hello world to /test.txt
	os.WriteFile("/home/alpine/test.txt", []byte("Hello World"), 0644)
	cmd := exec.Command("ctr", "images", "import", "/images/nginx.tar")
	cmd.Run()
	cmd = exec.Command("ctr", "run", "--net-host", "-d", "docker.io/library/nginx:latest", "app")
	cmd.Run()
}
