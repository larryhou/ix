package xpc

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestString(t *testing.T) {
	for _, s := range []string {
		`larryhou`,
		`larryhou1`,
		`larryhou12`,
		`larryhou123`,
		`larryhou1234`,
		`larryhou12345`,
	} {
		buf := &bytes.Buffer{}
		encoder := NewEncoder(buf)
		if err := encoder.string(s); err != nil {
			t.Fatal(err)
		}

		decoder := NewDecoder(buf)
		v, err := decoder.string()
		if err != nil {
			t.Fatal(err)
		}

		if v != s {
			t.Fatalf(`%s != %s`, v, s)
		}
	}
}

func TestCString(t *testing.T) {
	for _, s := range []string {
		`larryhou`,
		`larryhou1`,
		`larryhou12`,
		`larryhou123`,
		`larryhou1234`,
		`larryhou12345`,
	} {
		buf := &bytes.Buffer{}
		encoder := NewEncoder(buf)
		if err := encoder.cstring(s); err != nil {
			t.Fatal(err)
		}

		decoder := NewDecoder(buf)
		v, err := decoder.cstring()
		if err != nil {
			t.Fatal(err)
		}

		if v != s {
			t.Fatalf(`%s != %s`, v, s)
		}
	}
}

func TestDictionary(t *testing.T) {
	d := map[string]any{
		`name`:   `larryhou`,
		`gender`: `male`,
		`age`:    25,
	}

	buf := &bytes.Buffer{}
	encoder := NewEncoder(buf)
	if err := encoder.dictionary(d); err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%s\n", hex.EncodeToString(buf.Bytes()))

	decoder := NewDecoder(buf)
	v, err := decoder.dictionary()
	if err != nil {
		t.Fatal(err)
	}

	for k := range d {
		if value, ok := v[k]; !ok || value != d[k] {
			t.Fatalf(`%s %+v != %+v`, k, value, d[k])
		}
	}
}

func TestArray(t *testing.T) {
	d := []any{
		map[string]any{
			`name`:   `larryhou`,
			`gender`: `male`,
			`age`:    25,
		},
		`this is a description`,
		124354354,
		true,
		[]any{1, 2, 3, 4, 5, `hello`, false},
	}

	buf := &bytes.Buffer{}
	encoder := NewEncoder(buf)
	if err := encoder.array(d); err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%s\n", hex.EncodeToString(buf.Bytes()))

	decoder := NewDecoder(buf)
	v, err := decoder.array()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%#v\n", v)
}

func TestObject(t *testing.T) {
	d := map[string]any{
		`name`:   `larryhou`,
		`gender`: `male`,
		`age`:    25,
	}

	buf := &bytes.Buffer{}
	encoder := NewEncoder(buf)
	if err := encoder.object(d); err != nil {
		t.Fatal(err)
	}

	fmt.Printf("%s\n", hex.EncodeToString(buf.Bytes()))

	decoder := NewDecoder(buf)
	v, err := decoder.object()
	if err != nil {
		t.Fatal(err)
	}

	x, ok := v.(map[string]any)
	if !ok {
		t.Fatalf(`invalid dictionary cast`)
	}

	for k := range d {
		if value, ok := x[k]; !ok || value != d[k] {
			t.Fatalf(`%s %+v != %+v`, k, value, d[k])
		}
	}
}