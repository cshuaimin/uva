package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dustin/go-humanize"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/publicsuffix"
)

var (
	dataPath         = os.Getenv("HOME") + "/.local/share/uva-cli/"
	pdfPath          = dataPath + "pdf/"
	testDataPath     = dataPath + "test-data"
	loginInfoFile    = dataPath + "login-info.gob"
	problemsInfoFile = dataPath + "problems-info.gob"
	uvaURL, _        = url.Parse(baseURL)
)

const (
	baseURL  = "https://uva.onlinejudge.org"
	red      = "\033[0;31m"
	green    = "\033[0;32m"
	cyan     = "\033[1;36m"
	yellow   = "\033[0;33m"
	gray     = "\033[1;30m"
	hiyellow = "\033[1;33m"
	hiwhite  = "\033[1;37m"
	magenta  = "\033[1;35m"
	end      = "\033[0m"
)

func main() {
	app := cli.NewApp()
	app.Usage = "A cli tool to enjoy uva oj!"
	app.UsageText = "uva [command]"
	loadCookies := func(c *cli.Context) error {
		loadLoginInfo()
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:  "user",
			Usage: "manage account",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "l",
					Usage: "user login",
				},
				cli.BoolFlag{
					Name:  "L",
					Usage: "user logout",
				},
			},
			Action: user,
		},
		{
			Name:  "show",
			Usage: "show problem by name or id",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "p",
					Usage: "show input/output",
				},
			},
			Action: show,
			Before: loadCookies,
		},
		{
			Name:  "submit",
			Usage: "submit code",
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "i",
					Usage: "problem ID",
				},
			},
			Action: submitAndShowResult,
			Before: loadCookies,
		},
		{
			Name:   "test",
			Usage:  "test code locally",
			Action: testProgram,
		},
	}

	defer func() {
		if err := recover(); err != nil {
			fmt.Printf("%s%s%s\n", red, err, end)
			os.Exit(1)
		}
	}()

	// make data directories
	for _, path := range []string{dataPath, pdfPath, testDataPath} {
		if !exists(path) {
			if err := os.Mkdir(path, 0755); err != nil {
				panic(err)
			}
		}
	}
	app.Run(os.Args)
}

func spin(text string) func() {
	dots := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i := 0; i < len(dots); i++ {
		dots[i] = fmt.Sprintf("%s%s%s", green, dots[i], end)
	}
	text = fmt.Sprintf("%s%s%s", gray, text, end)
	stop := make(chan struct{})
	done := make(chan struct{})
	fmt.Printf("%s %s", dots[0], text)
	go func() {
		for i := 1; ; i++ {
			select {
			case <-time.After(100 * time.Millisecond):
				fmt.Printf("\r%s %s", dots[i%len(dots)], text)
			case <-stop:
				fmt.Printf("\r%s\r", strings.Repeat(" ", len(text)+2))
				done <- struct{}{}
				return
			}
		}
	}()
	return func() {
		stop <- struct{}{}
		// Wait and make sure the spinner is erased.
		<-done
	}
}

func exists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

type problemInfo struct {
	Title            string
	ID               int
	TrueID           int
	TotalSubmissions int
	Percentage       float32
}

func crawlProblemsInfo() map[int]problemInfo {
	// First, get all volumes' URL from two categories - "Problem Set Volumes" and "Contest Volumes".
	volumesChan := make(chan string)
	var volumesWaitGroup sync.WaitGroup
	getVolumes := func(category int) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("%s%s%s\n", red, err, end)
				os.Exit(1)
			}
		}()

		resp, err := http.Get(fmt.Sprintf("%s/index.php?option=com_onlinejudge&Itemid=8&category=%d", baseURL, category))
		if err != nil {
			panic(err)
		}
		doc, err := goquery.NewDocumentFromResponse(resp)
		if err != nil {
			panic(err)
		}
		doc.Find("#col3_content_wrapper > table:nth-child(4) > tbody > tr > td > a").
			Each(func(i int, s *goquery.Selection) {
				href, ok := s.Attr("href")
				if !ok {
					panic("href not exists")
				}
				volumesChan <- href
			})
		volumesWaitGroup.Done()
	}
	volumesWaitGroup.Add(2)
	// Problem Set Volumes (100...1999)
	go getVolumes(1)
	// Contest Volumes (10000...)
	go getVolumes(2)
	go func() {
		volumesWaitGroup.Wait()
		close(volumesChan)
	}()

	// Second, get all problems' information from each volume.
	problemsChan := make(chan problemInfo)
	var problemsWaitGroup sync.WaitGroup
	// \s does not match &nbsp;
	titleRegex, _ := regexp.Compile("(\\d+)\u00A0-\u00A0(.+)")
	trueIDRegex, _ := regexp.Compile(`.+problem=(\d+)`)
	getProblems := func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("%s%s%s\n", red, err, end)
				os.Exit(1)
			}
		}()

		for volumeURL := range volumesChan {
			resp, err := http.Get(fmt.Sprintf("%s/%s", baseURL, volumeURL))
			if err != nil {
				panic(err)
			}
			doc, err := goquery.NewDocumentFromResponse(resp)
			if err != nil {
				panic(err)
			}
			doc.Find("#col3_content_wrapper > table:nth-child(4) > tbody > tr[class!=sectiontableheader]").
				Each(func(i int, s *goquery.Selection) {
					var problem problemInfo
					ele := s.Find("td:nth-child(3) > a")
					match := titleRegex.FindSubmatch([]byte(ele.Text()))[1:]
					problem.ID, _ = strconv.Atoi(string(match[0]))
					problem.Title = string(match[1])
					href, ok := ele.Attr("href")
					if !ok {
						panic("href not exists")
					}
					problem.TrueID, _ = strconv.Atoi(string(trueIDRegex.FindSubmatch([]byte(href))[1]))
					problem.TotalSubmissions, _ = strconv.Atoi(s.Find("td:nth-child(4)").Text())
					text := s.Find("td:nth-child(5) > div > div:nth-child(2)").Text()
					p, _ := strconv.ParseFloat(text[:len(text)-1], 32)
					problem.Percentage = float32(p)
					problemsChan <- problem
				})
		}
		problemsWaitGroup.Done()
	}
	const WORKERS = 8
	problemsWaitGroup.Add(WORKERS)
	for i := 0; i < WORKERS; i++ {
		go getProblems()
	}
	go func() {
		problemsWaitGroup.Wait()
		close(problemsChan)
	}()

	// Finally, collect all the problems.
	problems := make(map[int]problemInfo)
	defer spin("Downloading problem list")()
	for p := range problemsChan {
		problems[p.ID] = p
	}
	return problems
}

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

type loginInfo struct {
	// Export these fields so that gob can dump them.
	Username string
	Cookies  []*http.Cookie
}

func login() {
	var username string
	fmt.Print("Username: ")
	fmt.Scanln(&username)
	fmt.Print("Password: ")
	password, err := terminal.ReadPassword(0)
	fmt.Print("\n")
	if err != nil {
		panic(err)
	}

	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		panic(err)
	}
	http.DefaultClient.Jar = jar

	stop := spin("Signing in uva.onlinejudge.org")
	resp, err := http.Get(baseURL)
	if err != nil {
		panic(err)
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		panic(err)
	}
	form := url.Values{}
	doc.Find("#mod_loginform > table > tbody > tr:nth-child(1) > td > input").
		Each(func(i int, s *goquery.Selection) {
			name, _ := s.Attr("name")
			value := s.AttrOr("value", "")
			form.Set(name, value)
		})
	form.Set("username", username)
	form.Set("passwd", string(password))
	r, err := http.PostForm(
		baseURL+"/index.php?option=com_comprofiler&task=login", form)
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	const failed = "Incorrect username or password"
	if strings.Contains(string(body), failed) {
		panic(failed)
	}
	f, err := os.Create(loginInfoFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	info := loginInfo{
		Username: username,
		Cookies:  http.DefaultClient.Jar.Cookies(uvaURL),
	}
	if err := gob.NewEncoder(f).Encode(info); err != nil {
		panic(err)
	}
	stop()
	fmt.Printf("Successfully login as %s%s%s\n", hiyellow, username, end)
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

func crawlTestData(pid int) (string, string) {
	defer spin("Downloading test cases")()
	problemHomePage := fmt.Sprintf("https://www.udebug.com/UVa/%d", pid)
	doc, err := goquery.NewDocument(problemHomePage)
	if err != nil {
		panic(err)
	}
	inputID, ok := doc.Find("a.input_desc").Attr("data-id")
	if !ok {
		panic("no input found")
	}
	resp, err := http.PostForm(
		"https://www.udebug.com/udebug-custom-get-selected-input-ajax",
		url.Values{"input_nid": {inputID}},
	)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		panic(err)
	}
	input := m["input_value"]
	form := url.Values{}
	doc.Find("#udebug-custom-problem-view-input-output-form input").Each(func(i int, s *goquery.Selection) {
		form.Set(s.AttrOr("name", ""), s.AttrOr("value", ""))
	})
	form.Set("input_data", input)
	resp, err = http.PostForm(problemHomePage, form)
	if err != nil {
		panic(err)
	}
	doc, err = goquery.NewDocumentFromResponse(resp)
	if err != nil {
		panic(err)
	}
	return input, doc.Find("#edit-output-data").Text()
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
