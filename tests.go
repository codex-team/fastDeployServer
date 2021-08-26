package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
)

// assert fails the test if the condition is false.
func assert(tb testing.TB, condition bool, msg string, v ...interface{}) {
	if !condition {
		_, file, line, _ := runtime.Caller(1)
		fmt.Printf("\033[31m%s:%d: "+msg+"\033[39m\n\n", append([]interface{}{filepath.Base(file), line}, v...)...)
		tb.FailNow()
	}
}

// test equality of two string arrays without order checking
func testUnorderedEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aset := make(map[string]struct{})
	for _, v := range a {
		aset[v] = struct{}{}
	}
	for _, v := range b {
		if _, ok := aset[v]; !ok {
			return false
		}
	}
	return true
}