//go:build darwin
package remotepair

//#cgo LDFLAGS: -Llib/darwin
//#cgo CXXFLAGS: -fvisibility-inlines-hidden -DCASE_INSENSITIVE -stdlib=libc++
import "C"
