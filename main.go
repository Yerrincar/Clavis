package main

import (
	"Clavis/src/networking"
	"log"
)

func main() {
	handler := networking.NewHandler()

	err := handler.Server()
	if err != nil {
		log.Fatal(err)
	}

}
