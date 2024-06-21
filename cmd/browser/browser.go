package main

import (
	"github.com/larryhou/gomobiledevice3/api/bonjour"
	"log"
)

func main() {
	addr, err := bonjour.TCPAddr(bonjour.RemotedServiceName)
	if err != nil {panic(err)}

	log.Printf(`%s`, addr)
}