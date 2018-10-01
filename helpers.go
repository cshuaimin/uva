package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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

type pdfInfo struct {
	pinfo        problemInfo
	description  string
	input        string
	output       string
	sampleInput  string
	sampleOutput string
}

func parsePdf(pid int) pdfInfo {
	var pdf pdfInfo
	pdf.pinfo = getProblemInfo(pid)
	title := strings.Replace(pdf.pinfo.Title, " ", "-", -1)
	pdfFile := fmt.Sprintf("%s%d.%s.pdf", pdfPath, pid, title)
	var f *os.File
	var err error

	if exists(pdfFile) {
		f, err = os.Open(pdfFile)
		if err != nil {
			panic(err)
		}
	} else {
		f, err = os.Create(pdfFile)
		if err != nil {
			panic(err)
		}
		stop := spin("Downloading " + title)
		resp, err := http.Get(fmt.Sprintf("%s/external/%d/p%d.pdf", baseURL, pid/100, pid))
		if err != nil {
			panic(err)
		}
		if _, err := io.Copy(f, resp.Body); err != nil {
			panic(err)
		}
		resp.Body.Close()
		stop()
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			panic(err)
		}
	}
	defer f.Close()

	cmd := exec.Command("pdftotext", "-", "-")
	cmd.Stdin = f
	out, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	defer out.Close()
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	bs, err := ioutil.ReadAll(out)
	if err != nil {
		panic(err)
	}
	pdfRegex, _ := regexp.Compile("(?s)(.+)\nInput\n(.+)\nOutput\n(.+)\nSample Input\n(.+)\nSample Output\n(.+)")
	res := pdfRegex.FindSubmatch(bs)[1:]
	indent := func(b []byte) string {
		return "       " + strings.Replace(string(b), "\n", "\n       ", -1)
	}
	pdf.description = indent(res[0])
	pdf.input = indent(res[1])
	pdf.output = indent(res[2])
	pdf.sampleInput = indent(res[3])
	pdf.sampleOutput = indent(res[4])
	return pdf

}
