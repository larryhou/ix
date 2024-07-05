package util

import (
	"fmt"
	"log"
)

func Return[T any](v *T, err error) (*T, error) {
	if err != nil {return nil, err}
	return v, nil
}

func Cast[T any](v any, err error) (r T, e error) {
	e = err
	if e != nil {return}
	r, ok := v.(T)
	if !ok && v != nil {
		e = fmt.Errorf(`CAST: %+v`, v)
		return
	}

	return
}

func Print(v any, err error) {
	if err != nil {
		log.Printf("%v", err)
	} else {
		log.Printf(`%+v`, v)
	}
}
