package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"
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

func parseFilename(s string) (pid int, name string, ext string) {
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
	ext = string(match[3])
	return
}

func (info problemInfo) getFilename(ext string) string {
	slug := strings.Replace(info.Title, " ", "-", -1)
	return fmt.Sprintf("%d.%s.%s", info.ID, slug, ext)
}

func download(url, file, msg string) {
	defer spin(msg)()
	f, err := os.Create(file)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	io.Copy(f, resp.Body)
}

func getTestCmd(ext string, sourceFile string) (compile []string, run []string) {
	configFile := dataPath + "config.yml"
	if !exists(configFile) {
		download("https://github.com/cshuaimin/uva/raw/master/config.yml", configFile, "Downloading default config.yml")
	}
	f, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	var m map[string]map[string][]string
	if err = yaml.NewDecoder(f).Decode(&m); err != nil {
		panic(err)
	}
	compile = m[ext]["compile"]
	run = m[ext]["run"]
	render := func(sl []string) {
		for i, v := range sl {
			if v == "{}" {
				sl[i] = sourceFile
			}
		}
	}
	render(compile)
	render(run)
	return
}
