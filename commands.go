package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	humanize "github.com/dustin/go-humanize"
	"github.com/urfave/cli"
)

func submit(problemID int, file string) string {
	category := problemID / 100
	info := getProblemInfo(problemID)
	problemID = info.TrueID
	form := url.Values{
		"problemid": {strconv.Itoa(problemID)},
		"category":  {strconv.Itoa(category)},
		"language":  {"3"}, // TODO
	}
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	code, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}
	form.Set("code", string(code))

	// Prevent HTTP 301 redirect
	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer func() { http.DefaultClient.CheckRedirect = nil }()
	defer spin("Sending code to judge")()
	resp, err := http.PostForm(baseURL+
		"/index.php?option=com_onlinejudge&Itemid=8&page=save_submission", form)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	location := resp.Header["Location"][0]
	sidRegex, _ := regexp.Compile(`Submission\+received\+with\+ID\+(\d+)`)
	submitID := string(sidRegex.FindSubmatch([]byte(location))[1])
	return submitID
}

func getResult(submitID string) (result, runTime string) {
	resp, err := http.Get(baseURL + "/index.php?option=com_onlinejudge&Itemid=9")
	if err != nil {
		panic(err)
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		panic(err)
	}
	row := doc.Find("#col3_content_wrapper > table:nth-child(3) > tbody > tr:nth-child(2) > td")
	if row.First().Text() != submitID {
		panic("not latest submit")
	}
	return strings.TrimSpace(row.Eq(3).Text()), row.Eq(5).Text()
}

const (
	ansic = iota + 1
	java
	cpp
	pascal
	cpp11
	python3
)

func submitAndShowResult(c *cli.Context) {
	if c.NArg() == 0 {
		panic("filename required")
	}
	pid := c.Int("i")
	if pid == 0 {
		//
	}
	sid := submit(pid, c.Args().Get(0))
	stop := spin("Waiting for judge result")
	const judging = "In judge queue"
	result := judging
	var runTime string
	for result == judging {
		result, runTime = getResult(sid)
		time.Sleep(1 * time.Second)
	}
	stop()

	if result == "Accepted" {
		fmt.Printf("%s✔ Accepted (%ss)%s\n", cyan, runTime, end)
	} else {
		fmt.Printf("%s✘ %s%s\n", red, result, end)
	}
}

func user(c *cli.Context) {
	if c.Bool("l") {
		login()
		return
	}
	if c.Bool("L") {
		if err := os.Remove(loginInfoFile); err != nil {
			panic(err)
		}
		return
	}

	fmt.Printf("You are now logged in as %s%s%s\n", hiyellow, loadLoginInfo().Username, end)
}

func show(c *cli.Context) {
	if c.NArg() == 0 {
		panic("problem name or id required")
	}
	pid, err := strconv.Atoi(c.Args().First()) // TODO: prohlem name
	if err != nil {
		panic(err)
	}
	pdf := parsePdf(pid)

	title := fmt.Sprintf("%d - %s", pid, pdf.pinfo.Title)
	padding := strings.Repeat(" ", (108-len(title))/2)
	fmt.Printf("%s%s%s%s\n\n", padding, hiwhite, title, end)

	fmt.Printf("%sStatistics%s\n", hiwhite, end)
	fmt.Printf("       * Rate: %.1f %%\n", pdf.pinfo.Percentage)
	accepted := humanize.Bytes(uint64(float32(pdf.pinfo.TotalSubmissions) * pdf.pinfo.Percentage / 100))
	fmt.Printf("       * Total Accepted: %s\n", accepted[:len(accepted)-1])
	submissions := humanize.Bytes(uint64(pdf.pinfo.TotalSubmissions))
	fmt.Printf("       * Total Submissions: %s\n\n", submissions[:len(submissions)-1])

	fmt.Printf("%sDescription%s\n", hiwhite, end)
	fmt.Println(pdf.description)

	if c.Bool("p") {
		fmt.Printf("%sInput%s\n", hiwhite, end)
		fmt.Println(pdf.input)

		fmt.Printf("%sOutput%s\n", hiwhite, end)
		fmt.Println(pdf.output)

		fmt.Printf("%sSample Input%s\n", hiwhite, end)
		fmt.Println(pdf.sampleInput)

		fmt.Printf("%sSample Output%s\n", hiwhite, end)
		fmt.Println(pdf.sampleOutput)
	}
}

func testProgram(c *cli.Context) {
	if c.NArg() == 0 {
		panic("filename required")
	}
	file := c.Args().First()
	pid, name, lang := parseFilename(file)
	binFilename := fmt.Sprintf("%d.%s", pid, name)

	// compile source code
	cmd := exec.Command("g++", "-Wall", "-fdiagnostics-color=always", "-o", binFilename, file)
	stop := spin("Compiling")
	out, err := cmd.CombinedOutput()
	stop()
	if err != nil {
		panic(err)
	}
	if len(out) != 0 {
		fmt.Printf("%s✘ Warnings%s\n", magenta, end)
		fmt.Print(string(out))
	}

	// get test case from udebug.com
	input, output := getTestData(pid)

	// run the program with test case
	cmd = exec.Command("./" + binFilename)
	tmpfile, err := ioutil.TempFile("", binFilename+".output-")
	if err != nil {
		panic(err)
	}
	defer os.Remove(tmpfile.Name())
	cmd.Stdout = tmpfile
	cmd.Stdin = strings.NewReader(input)
	stop = spin("running tests")
	start := time.Now()
	if err = cmd.Run(); err != nil {
		panic(err)
	}
	runTime := time.Since(start)
	stop()

	// compare the output with the answer
	cmd = exec.Command("diff", "-Z", "--color=always", tmpfile.Name(), "-")
	cmd.Stdin = strings.NewReader(output)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	err = cmd.Run()
	if err != nil {
		if v, ok := err.(*exec.ExitError); !ok {
			panic(v)
		}
	}
	diff := string(buf.Bytes())
	if len(diff) != 0 {
		fmt.Printf("%s✘ Wrong answer%s\n", red, end)
		fmt.Print(diff)
	} else {
		fmt.Printf("%s✔ Accepted (%.2fs)%s\n", cyan, float32(runTime)/float32(time.Second), end)
	}
	lang = lang
}
