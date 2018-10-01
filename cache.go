package main

import (
	"encoding/gob"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/net/publicsuffix"
)

func getProblemInfo(pid int) problemInfo {
	var problems map[int]problemInfo
	if exists(problemsInfoFile) {
		f, err := os.Open(problemsInfoFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := gob.NewDecoder(f).Decode(&problems); err != nil {
			panic(err)
		}
	} else {
		problems = crawlProblemsInfo()
		f, err := os.Create(problemsInfoFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := gob.NewEncoder(f).Encode(problems); err != nil {
			panic(err)
		}
	}
	r, ok := problems[pid]
	if !ok {
		panic("problem not found")
	}
	return r
}

func getTestData(pid int) (input string, output string) {
	testDataFile := fmt.Sprintf("%s/%d.gob", testDataPath, pid)
	if exists(testDataFile) {
		f, err := os.Open(testDataFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		dec := gob.NewDecoder(f)
		if err = dec.Decode(&input); err != nil {
			panic(err)
		}
		if err = dec.Decode(&output); err != nil {
			panic(err)
		}
	} else {
		input, output = crawlTestData(pid)
		f, err := os.Create(testDataFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		enc := gob.NewEncoder(f)
		if err = enc.Encode(input); err != nil {
			panic(err)
		}
		if err = enc.Encode(output); err != nil {
			panic(err)
		}
	}
	return
}

func loadLoginInfo() loginInfo {
	if !exists(loginInfoFile) {
		panic("you are not logged in yet")
	}

	f, err := os.Open(loginInfoFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	var info loginInfo
	if err := gob.NewDecoder(f).Decode(&info); err != nil {
		panic(err)
	}
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		panic(err)
	}
	jar.SetCookies(uvaURL, info.Cookies)
	http.DefaultClient.Jar = jar
	return info
}

func getProblemDescription(pid int, title string) string {
	title = strings.Replace(title, " ", "-", -1)
	pdfFile := fmt.Sprintf("%s%d.%s.pdf", pdfPath, pid, title)
	if !exists(pdfFile) {
		downloadProblemPdf(pid, pdfFile)
	}
	pdf, err := exec.Command("pdftotext", pdfFile, "-").Output()
	if err != nil {
		panic(err)
	}
	return string(pdf)
}
