package main

import (
	"github.com/larryhou/ix/api/tunneld"
	"log"
)

func main() {
	log.Fatal(tunneld.Run())
}