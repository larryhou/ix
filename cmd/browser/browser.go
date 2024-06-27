package main

import (
	"github.com/larryhou/j3idevice/api/bonjour"
	"log"
)

func main() {
	addr, err := bonjour.TCPAddr(bonjour.RemotedServiceName)
	if err != nil {panic(err)}

	log.Printf(`%s`, addr)
}