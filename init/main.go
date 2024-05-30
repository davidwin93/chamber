package main

import (
	"log"
	"net/http"
)

func main() {
	// write hello world to /test.txt
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	log.Fatal(http.ListenAndServe(":22", nil))
}
