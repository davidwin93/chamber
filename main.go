package main

import (
	"chamber/internal/server"
	"log"
)

func main() {
	// Listen for incoming connections
	api := &server.APIServer{}
	log.Println(api.StartAPIServer("8070"))
}
