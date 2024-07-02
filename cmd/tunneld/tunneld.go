package main

import (
	"github.com/larryhou/j3idevice/api/tunneld"
	"log"
)

func main() {
	log.Fatal(tunneld.Run())
}