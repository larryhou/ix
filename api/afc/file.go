package afc

import (
	"errors"
	"io"
)

type FileHandle struct {
	name string
	fd uint64
	sv *Service
}

func (x *FileHandle) FileReader() (io.Reader, error) {
	fr := &fileReader{
		FileHandle: x,
	}

	if err := fr.prepare(); err != nil {
		return nil, err
	}

	return fr, nil
}

func (x *FileHandle) FileWriter(n int64) (io.Writer, error) {
	fw := &fileWriter{
		FileHandle: x,
		Size:       n,
	}

	return fw, nil
}

func (x *FileHandle) Close() error {
	req := make([]byte, 8)
	x.sv.bo.PutUint64(req, x.fd)

	return x.sv.get(OpFileClose, req, nil)
}


type fileReader struct {
	*FileHandle
	*FileStat

	n int64
	r *io.LimitedReader
}

func (x *fileReader) prepare() error {
	st, err := x.sv.Stat(x.name)
	if err != nil {return err}
	if st.Ifmt != `S_IFREG` {
		return errors.New(x.name + ` isn't a file'`)
	}

	x.FileStat = st
	return nil
}

func (x *fileReader) Read(b []byte) (int, error) {
	if x.n == x.Size {return 0, io.EOF}
	if x.r == nil || x.r.N == 0 {
		req := make([]byte, 8 + 8)
		x.sv.bo.PutUint64(req[0:], x.fd)
		x.sv.bo.PutUint64(req[8:], uint64(x.Size - x.n))

		var r io.Reader
		if err := x.sv.get(OpRead, req, &r); err != nil {return 0, err}
		x.r = r.(*io.LimitedReader)
	}

	m := min(int64(len(b)), x.r.N)
	n, err := x.r.Read(b[:m])
	if err == nil {
		x.n += int64(n)
	}

	return n, err
}

type fileWriter struct {
	*FileHandle
	Size int64

	n int64
	r int64
}

func (x *fileWriter) Write(b []byte) (int, error) {
	if x.n == x.Size {return 0, nil}

	if x.r == 0 {
		x.r = min(x.Size - x.n, MaximumWriteSize)
		req := make([]byte, 8)
		x.sv.bo.PutUint64(req, x.fd)
		err := x.sv.send(OpWrite, &request{Args: req, Body: x.r})
		if err != nil {return 0, err}
	}

	k := min(int64(len(b)), x.r)
	n, err := x.sv.conn.Write(b[:k])
	if err == nil {
		x.r -= int64(n)
		x.n += int64(n)

		if x.r == 0 {
			_, err := x.sv.recv(nil, false)
			if err != nil {return 0, err}
		}
	}

	return n, err
}
