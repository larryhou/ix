package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/larryhou/j3idevice/api/tunnel/xpc"
)

func main() {
	buf := &bytes.Buffer{}

	msg := &xpc.Message{}
	err := xpc.Encode(buf, msg)
	if err != nil {panic(err)}

	fmt.Printf("%+v\n", hex.EncodeToString(buf.Bytes()))
}
