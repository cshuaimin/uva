package main

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
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
	userInfoFile     = dataPath + "user-info.gob"
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
	end      = "\033[0m"
)

type userInfo struct {
	// Export these fields so that gob can dump them.
	Username     string
	LoginCookies []*http.Cookie
}

type problemInfo struct {
	Title            string
	TotalSubmissions int
	Percentage       float32
	TrueID           int
}

func exists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
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

func volumeToCategory(volume int) int {
	switch {
	case volume <= 9:
		return volume + 2
	case 10 <= volume && volume <= 12:
		return volume + 235
	case 13 <= volume && volume <= 15:
		return volume + 433
	case volume == 16:
		return 825
	case volume == 17:
		return 859
	}
	return -1
}

func crawlProblemsInfo() []problemInfo {
	defer spin("Downloading problem list")()
	const VOLUMES = 17
	resultChan := make(chan problemInfo)
	var wg sync.WaitGroup
	wg.Add(VOLUMES)
	// \s does not match &nbsp;
	titleRegex, _ := regexp.Compile("\\d+\u00A0-\u00A0(.+)")
	trueIDRegex, _ := regexp.Compile(`.+problem=(\d+)`)
	for i := 1; i <= VOLUMES; i++ {
		go func(volume int) {
			defer func() {
				if err := recover(); err != nil {
					fmt.Printf("%s%s%s\n", red, err, end)
					os.Exit(1)
				}
			}()
			category := volumeToCategory(volume)
			resp, err := http.Get(fmt.Sprintf("%s%s%s", baseURL,
				"/index.php?option=com_onlinejudge&Itemid=8&category=",
				strconv.Itoa(category)))
			if err != nil {
				panic(err)
			}
			doc, err := goquery.NewDocumentFromResponse(resp)
			doc.Find("#col3_content_wrapper > table:nth-child(4) > tbody > tr[class^=sectiontableentry]").
				Each(func(i int, s *goquery.Selection) {
					var problem problemInfo
					ele := s.Find("td:nth-child(3) > a")
					problem.Title = string(titleRegex.FindSubmatch([]byte(ele.Text()))[1])
					href, ok := ele.Attr("href")
					if !ok {
						panic("href not exists")
					}
					tid := string(trueIDRegex.FindSubmatch([]byte(href))[1])
					problem.TrueID, _ = strconv.Atoi(tid)
					problem.TotalSubmissions, _ = strconv.Atoi(s.Find("td:nth-child(4)").Text())
					text := s.Find("td:nth-child(5) > div > div:nth-child(2)").Text()
					p, _ := strconv.ParseFloat(text[:len(text)-1], 32)
					problem.Percentage = float32(p)
					resultChan <- problem
				})
			wg.Done()
		}(i)
	}
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	var problems []problemInfo
	for p := range resultChan {
		problems = append(problems, p)
	}
	sort.Slice(problems, func(i, j int) bool {
		return problems[i].TrueID < problems[j].TrueID
	})

	return problems
}

func getProblemInfo(pid int) (problemInfo, error) {
	var problems []problemInfo
	if exists(problemsInfoFile) {
		f, err := os.Open(problemsInfoFile)
		if err != nil {
			return problemInfo{}, err
		}
		defer f.Close()
		if err := gob.NewDecoder(f).Decode(&problems); err != nil {
			return problemInfo{}, err
		}
	} else {
		problems = crawlProblemsInfo()
		f, err := os.Create(problemsInfoFile)
		if err != nil {
			return problemInfo{}, err
		}
		defer f.Close()
		if err := gob.NewEncoder(f).Encode(problems); err != nil {
			return problemInfo{}, err
		}
	}
	return problems[pid-100], nil
}

func doLogin(username, password string) error {
	defer spin("Signing in uva.onlinejudge.org")()
	resp, err := http.Get(baseURL)
	if err != nil {
		return err
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return err
	}
	form := url.Values{}
	doc.Find("#mod_loginform > table > tbody > tr:nth-child(1) > td > input").
		Each(func(i int, s *goquery.Selection) {
			name, _ := s.Attr("name")
			value := s.AttrOr("value", "")
			form.Set(name, value)
		})
	form.Set("username", username)
	form.Set("passwd", password)
	r, err := http.PostForm(
		baseURL+"/index.php?option=com_comprofiler&task=login", form)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	const failed = "Incorrect username or password"
	if strings.Contains(string(body), failed) {
		return errors.New(failed)
	}
	return nil
}

func login() error {
	if !exists(dataPath) {
		if err := os.Mkdir(dataPath, 0755); err != nil {
			return err
		}
	}
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return err
	}
	http.DefaultClient.Jar = jar

	if !exists(userInfoFile) {
		var username string
		fmt.Print("Username: ")
		fmt.Scanln(&username)
		fmt.Print("Password: ")
		password, err := terminal.ReadPassword(0)
		fmt.Print("\n")
		if err != nil {
			return err
		}
		if err := doLogin(username, string(password)); err != nil {
			return err
		}
		f, err := os.Create(userInfoFile)
		if err != nil {
			return err
		}
		defer f.Close()
		user := userInfo{
			Username:     username,
			LoginCookies: jar.Cookies(uvaURL),
		}
		if err := gob.NewEncoder(f).Encode(user); err != nil {
			return err
		}
		fmt.Printf("Successfully login as %s%s%s\n", hiyellow, username, end)
	} else {
		f, err := os.Open(userInfoFile)
		if err != nil {
			return err
		}
		var user userInfo
		if err := gob.NewDecoder(f).Decode(&user); err != nil {
			return err
		}
		jar.SetCookies(uvaURL, user.LoginCookies)
	}
	return nil
}

func submit(problemID int, file string) (string, error) {
	category := problemID / 100
	info, err := getProblemInfo(problemID)
	if err != nil {
		return "", err
	}
	problemID = info.TrueID
	form := url.Values{
		"problemid": {strconv.Itoa(problemID)},
		"category":  {strconv.Itoa(category)},
		"language":  {"3"}, // TODO
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	code, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
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
		return "", err
	}
	resp.Body.Close()
	location := resp.Header["Location"][0]
	sidRegex, _ := regexp.Compile(`Submission\+received\+with\+ID\+(\d+)`)
	submitID := string(sidRegex.FindSubmatch([]byte(location))[1])
	return submitID, nil
}

func getResult(submitID string) (result, runTime string, _ error) {
	resp, err := http.Get(baseURL + "/index.php?option=com_onlinejudge&Itemid=9")
	if err != nil {
		return "", "", err
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return "", "", err
	}
	row := doc.Find("#col3_content_wrapper > table:nth-child(3) > tbody > tr:nth-child(2) > td")
	if row.First().Text() != submitID {
		return "", "", errors.New("not latest submit")
	}
	return strings.TrimSpace(row.Eq(3).Text()), row.Eq(5).Text(), nil
}

func submitAndShowResult(c *cli.Context) error {
	if c.NArg() == 0 {
		return errors.New("filename required")
	}
	pid := c.Int("i")
	if pid == 0 {
		//
	}
	sid, err := submit(pid, c.Args().Get(0))
	if err != nil {
		return err
	}
	stop := spin("Waiting for judge result")
	const judging = "In judge queue"
	result := judging
	var runTime string
	for result == judging {
		result, runTime, err = getResult(sid)
		if err != nil {
			return err
		}
		time.Sleep(1 * time.Second)
	}
	stop()

	if result == "Accepted" {
		fmt.Printf("%s✔ Accepted (%ss)%s\n", cyan, runTime, end)
	} else {
		fmt.Printf("%s✘ %s%s\n", red, result, end)
	}
	return nil
}

func user(c *cli.Context) error {
	if c.Bool("l") {
		return login()
	}
	if c.Bool("L") {
		return os.Remove(userInfoFile)
	}

	if !exists(userInfoFile) {
		return errors.New("You are not logged in yet!")
	}
	f, err := os.Open(userInfoFile)
	if err != nil {
		return err
	}
	var user userInfo
	if err := gob.NewDecoder(f).Decode(&user); err != nil {
		return err
	}
	fmt.Printf("You are now logged in as %s%s%s\n", hiyellow, user.Username, end)
	return nil
}

type pdfInfo struct {
	pinfo        problemInfo
	description  string
	input        string
	output       string
	sampleInput  string
	sampleOutput string
}

func parsePdf(pid int) (pdfInfo, error) {
	var pdf pdfInfo
	if !exists(pdfPath) {
		if err := os.Mkdir(pdfPath, 0755); err != nil {
			return pdf, err
		}
	}
	var err error
	pdf.pinfo, err = getProblemInfo(pid)
	if err != nil {
		return pdf, err
	}
	title := strings.Replace(pdf.pinfo.Title, " ", "-", -1)
	pdfFile := fmt.Sprintf("%s%d.%s.pdf", pdfPath, pid, title)
	var f *os.File

	if exists(pdfFile) {
		f, err = os.Open(pdfFile)
		if err != nil {
			return pdf, err
		}
	} else {
		f, err = os.Create(pdfFile)
		if err != nil {
			return pdf, err
		}
		stop := spin("Downloading " + title)
		resp, err := http.Get(fmt.Sprintf("%s/external/%d/p%d.pdf", baseURL, pid/100, pid))
		if err != nil {
			return pdf, err
		}
		if _, err := io.Copy(f, resp.Body); err != nil {
			return pdf, err
		}
		resp.Body.Close()
		stop()
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return pdf, err
		}
	}
	defer f.Close()

	cmd := exec.Command("pdftotext", "-", "-")
	cmd.Stdin = f
	out, err := cmd.StdoutPipe()
	if err != nil {
		return pdf, err
	}
	defer out.Close()
	if err := cmd.Start(); err != nil {
		return pdf, err
	}
	bs, err := ioutil.ReadAll(out)
	if err != nil {
		return pdf, err
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
	return pdf, nil

}

func show(c *cli.Context) error {
	if c.NArg() == 0 {
		return errors.New("problem name or id required")
	}
	pid, err := strconv.Atoi(c.Args().First()) // TODO: prohlem name
	if err != nil {
		return err
	}
	pdf, err := parsePdf(pid)
	if err != nil {
		return err
	}

	title := fmt.Sprintf("%d - %s", pid, pdf.pinfo.Title)
	padding := strings.Repeat(" ", (108-len(title))/2)
	fmt.Printf("%s%s%s%s\n\n", padding, hiwhite, title, end)

	fmt.Printf("%sStatistics%s\n", hiwhite, end)
	fmt.Printf("       * Rate: %.1f %%\n", pdf.pinfo.Percentage)
	accepted := humanize.Bytes(uint64(float32(pdf.pinfo.TotalSubmissions) * pdf.pinfo.Percentage))
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

	return nil
}

func main() {
	app := cli.NewApp()
	app.Usage = "A cli tool to enjoy uva oj!"
	app.UsageText = "uva [command]"
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
		},
	}
	if err := login(); err != nil {
		fmt.Printf("%s%s%s\n", red, err, end)
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%s%s%s\n", red, err, end)
	}
}
