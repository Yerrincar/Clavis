package main

import (
	"Clavis/src/networking"
	"context"
	"log"
	"time"
)

func main() {
	handler := networking.NewHandler()

	handler.Dispatch.Store.StartActiveExpireCycle(context.Background(), 20*time.Second)

	err := handler.Server()
	if err != nil {
		log.Fatal(err)
	}

}
