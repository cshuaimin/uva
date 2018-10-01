package main

import (
	"os"
	"regexp"
	"strconv"
)

const (
	ansic = iota + 1
	java
	cpp
	pascal
	cpp11
	python3
)

func exists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

func parseFilename(s string) (pid int, name string, lang int) {
	regex := regexp.MustCompile(`(\d+)\.([\w+-_]+)\.(\w+)`)
	match := regex.FindSubmatch([]byte(s))
	if len(match) != 4 {
		panic("filename pattern does not match")
	}
	pid, err := strconv.Atoi(string(match[1]))
	if err != nil {
		panic(err)
	}
	name = string(match[2])
	switch string(match[3]) {
	case "c":
		lang = ansic
	case "java":
		lang = java
	case "cc", "cpp":
		lang = cpp11
	case "pas":
		lang = pascal
	case "py":
		lang = python3
	}
	return
}
