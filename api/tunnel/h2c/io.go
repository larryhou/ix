package h2c

import (
	"io"
	"log"
)

func NewWriter(w io.Writer) io.Writer {
	return &fullWriter{w: w}
}

func NewReader(r io.Reader) io.Reader {
	return &fullReader{reader: r}
}

type fullWriter struct {
	w io.Writer
}

func (x *fullWriter) Write(b []byte) (int, error) {
	n := len(b)
	for t := 0; t < n; {
		k, err := x.w.Write(b[t:])
		if err != nil {
			log.Printf(`Fw: %v`, err)
			return 0, err
		}
		t += k
	}
	return n, nil
}

type fullReader struct {
	reader io.Reader
}

func (x *fullReader) Read(b []byte) (int, error) {
	n := len(b)
	for t := 0; t < n; {
		k, err := x.reader.Read(b[t:])
		if err != nil {return 0, err}
		t += k
	}
	return n, nil
}
