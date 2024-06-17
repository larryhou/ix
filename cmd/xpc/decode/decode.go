package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/larryhou/gomobiledevice3/api/tunnel/xpc"
	"io"
	"os"
)

func dump(msg any) {
	j := json.NewEncoder(os.Stdout)
	j.SetIndent(``, `    `)
	j.SetEscapeHTML(false)
	j.Encode(msg)
}

func main() {
	f, err := os.Open(os.Args[1])
	if err != nil {panic(err)}
	raw, err := io.ReadAll(hex.NewDecoder(f))
	if err != nil {panic(err)}

	msg := &xpc.Message{}
	err = xpc.Decode(bytes.NewReader(raw), msg)
	if err != nil {panic(err)}

	fmt.Printf("%+v %+v\n", msg, msg.Payload)
	dump(msg.Payload.Data)
}
