package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

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

var symbol = regexp.MustCompile(`[^\w\s-]`)
var spaces = regexp.MustCompile(`\s+`)
var filename = regexp.MustCompile(`(\d+)\.([\w-]+)\.(\w+)`)

func parseFilename(s string) (pid int, name string, ext string) {
	match := filename.FindStringSubmatch(s)
	if len(match) != 4 {
    panic("filename pattern does not match. help: please create file with `uva touch` command")
	}
	pid, err := strconv.Atoi(match[1])
	if err != nil {
		panic(err)
	}
	name = string(match[2])
	ext = string(match[3])
	return
}

func (info problemInfo) getFileName(ext string) string {
	slug := symbol.ReplaceAllString(info.Title, "")
	slug = spaces.ReplaceAllString(slug, "-")
	return fmt.Sprintf("%d.%s.%s", info.ID, slug, ext)
}

func download(url, file, msg string) {
	defer spin(msg)()
	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	f, err := os.Create(file)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if _, err = io.Copy(f, resp.Body); err != nil {
		panic(err)
	}
}

var config struct {
	Test map[string]struct {
		Compile, Run []string
	}
	Lang string
}

func loadConfig() {
	configFile := dataPath + "config.yml"
	if !exists(configFile) {
		download("https://github.com/cshuaimin/uva/raw/master/config.yml", configFile, "Downloading default config.yml")
	}
	f, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	if err = yaml.NewDecoder(f).Decode(&config); err != nil {
		panic(err)
	}
}

func renderCmd(cmd []string, sourceFile string) *exec.Cmd {
	if len(cmd) > 0 {
		for i, v := range cmd {
			if v == "{}" {
				cmd[i] = sourceFile
			}
		}
		return exec.Command(cmd[0], cmd[1:]...)
	}
	return nil
}

// line-by-line diff
func diff(text1, text2, label1, label2 string, sep string) (diff string, same bool) {
	lines1 := strings.Split(text1, "\n")
	lines2 := strings.Split(text2, "\n")
	same = true
	longest := 0
	original_lens := make([]int, len(lines1))
	lineno := 0
	for ; lineno < len(lines1) && lineno < len(lines2); lineno++ {
		if l := len(lines1[lineno]); l > longest {
			longest = l
		}
		original_lens[lineno] = len(lines1[lineno])
		// ignore spaces at line end
		lines1[lineno] = strings.TrimRight(lines1[lineno], " ")
		lines2[lineno] = strings.TrimRight(lines2[lineno], " ")
		words1 := strings.Split(lines1[lineno], sep)
		words2 := strings.Split(lines2[lineno], sep)
		idx := 0
		for ; idx < len(words1) && idx < len(words2); idx++ {
			if words1[idx] != words2[idx] {
				same = false
				// this changes the string length
				words1[idx] = colored(words1[idx], green, 0)
				words2[idx] = colored(words2[idx], red, 0)
			}
		}
		if len(words1) != len(words2) {
			same = false
		}
		for ; idx < len(words1); idx++ {
			words1[idx] = colored(words1[idx], green, 0)
		}
		for ; idx < len(words2); idx++ {
			words2[idx] = colored(words2[idx], red, 0)
		}

		lines1[lineno] = strings.Join(words1, sep)
		lines2[lineno] = strings.Join(words2, sep)
	}
	if len(lines1) != len(lines2) {
		same = false
	}
	for ; lineno < len(lines1); lineno++ {
		if l := len(lines1[lineno]); l > longest {
			longest = l
		}
		lines1[lineno] = colored(lines1[lineno], green, 0)
	}
	for ; lineno < len(lines2); lineno++ {
		lines2[lineno] = colored(lines2[lineno], red, 0)
	}
	if same {
		return "", true
	}

	if l := utf8.RuneCountInString(label1); l > longest {
		longest = l
	}
	var buf strings.Builder
	buf.WriteString(colored(label1, green, 0))
	buf.WriteString(strings.Repeat(" ", longest-utf8.RuneCountInString(label1)+2))
	buf.WriteString(colored(label2, red, 0))
	buf.WriteString("\n")

	for lineno = 0; lineno < len(lines1) && lineno < len(lines2); lineno++ {
		buf.WriteString(lines1[lineno])
		buf.WriteString(strings.Repeat(" ", longest-original_lens[lineno]+2))
		buf.WriteString(lines2[lineno])
		buf.WriteString("\n")
	}
	for ; lineno < len(lines1); lineno++ {
		buf.WriteString(lines1[lineno])
		buf.WriteString("\n")
	}
	for ; lineno < len(lines2); lineno++ {
		buf.WriteString(strings.Repeat(" ", longest+2))
		buf.WriteString(lines2[lineno])
		buf.WriteString("\n")
	}
	return buf.String(), false
}
