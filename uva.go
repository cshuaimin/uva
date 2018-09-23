package main

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/urfave/cli"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/publicsuffix"
)

var (
	dataPath           = os.Getenv("HOME") + "/.local/share/uva-cli"
	cookieFile         = dataPath + "/cookiejar.gob"
	trueProblemIDsFile = dataPath + "/true-problem-ids.gob"
)

const baseURL = "https://uva.onlinejudge.org"

func exists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}

func spin(text string) func() {
	const GREEN = "\033[0;32m"
	const GRAY = "\033[1;30m"
	const END = "\033[0m"
	dots := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i := 0; i < len(dots); i++ {
		dots[i] = fmt.Sprintf("%s%s%s", GREEN, dots[i], END)
	}
	text = fmt.Sprintf("%s%s%s", GRAY, text, END)
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

func crawlTrueProblemIDs() []int {
	defer spin("Downloading problem list")()
	const VOLUMES = 17
	resultChan := make(chan int)
	var wg sync.WaitGroup
	wg.Add(VOLUMES)
	for i := 1; i <= VOLUMES; i++ {
		go func(volume int) {
			category := volumeToCategory(volume)
			resp, err := http.Get(fmt.Sprintf("%s%s%s", baseURL,
				"/index.php?option=com_onlinejudge&Itemid=8&category=",
				strconv.Itoa(category)))
			if err != nil {
				panic(err)
			}
			doc, err := goquery.NewDocumentFromResponse(resp)
			doc.Find("#col3_content_wrapper > table:nth-child(4) > tbody > tr").
				Each(func(i int, s *goquery.Selection) {
					href, ok := s.Attr("href")
					if !ok {
						panic("Failed to query selector")
					}
					start := len(href) - 1
					for href[start-1] != '=' {
						start--
					}
					pid, err := strconv.Atoi(href[start:])
					if err != nil {
						panic("Failed when atoi")
					}
					resultChan <- pid
				})
			wg.Done()
		}(i)
	}
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	var problemIDs []int
	for pid := range resultChan {
		problemIDs = append(problemIDs, pid)
	}
	sort.Ints(problemIDs)

	f, err := os.Create(trueProblemIDsFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	gob.NewEncoder(f).Encode(problemIDs)
	return problemIDs
}

func getTrueProblemID(pid int) int {
	var trueIDs []int
	if exists(trueProblemIDsFile) {
		f, err := os.Open(trueProblemIDsFile)
		if err != nil {
			panic(err)
		}
		if err := gob.NewDecoder(f).Decode(&trueIDs); err != nil {
			panic(err)
		}
	} else {
		trueIDs = crawlTrueProblemIDs()
	}
	return trueIDs[pid-100]
}

func login(username, password string) http.Client {
	uvaURL, _ := url.Parse(baseURL)

	doLogin := func(file *os.File) http.Client {
		defer spin("Signing in uva.onlinejudge.org")()
		jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		if err != nil {
			panic(err)
		}
		client := http.Client{Jar: jar}
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
				name, ok := s.Attr("name")
				if !ok {
					panic(err)
				}
				value := s.AttrOr("value", "")
				form.Set(name, value)
			})
		form.Set("username", username)
		form.Set("passwd", password)
		r, err := client.PostForm(
			baseURL+"/index.php?option=com_comprofiler&task=login", form)
		if err != nil {
			panic(err)
		}
		defer r.Body.Close()
		if r.StatusCode != 200 {
			panic(r)
		}
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		if strings.Contains(string(body), "Incorrect username or password") {
			fmt.Println("Incorrect username or password")
		}
		gob.NewEncoder(file).Encode(jar.Cookies(uvaURL))
		return client
	}

	if !exists(dataPath) {
		if err := os.Mkdir(dataPath, 0755); err != nil {
			panic(err)
		}
	}
	if !exists(cookieFile) {
		f, err := os.Create(cookieFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		return doLogin(f)
	}

	f, err := os.Open(cookieFile)
	if err != nil {
		panic(err)
	}
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		panic(err)
	}
	var cookies []*http.Cookie
	if err := gob.NewDecoder(f).Decode(&cookies); err != nil {
		panic(err)
	}
	jar.SetCookies(uvaURL, cookies)
	return http.Client{Jar: jar}
}

func submit(client http.Client, problemID int, file string) string {
	var category int = problemID / 100
	problemID = getTrueProblemID(problemID)
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
	form.Set("codeupl", string(code))

	// Prevent HTTP 301 redirect
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	defer spin("Sending code to judge")()
	resp, err := client.PostForm(baseURL+
		"/index.php?option=com_onlinejudge&Itemid=8&page=save_submission", form)
	if err != nil {
		panic(err)
	}
	b, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(b))
	resp.Body.Close()
	if resp.StatusCode != 301 {
		panic(resp)
	}
	location := resp.Header["Location"][0]
	start := len(location) - 1
	for location[start-1] != '+' {
		start--
	}
	submitID := location[start:]
	return submitID
}

func main() {
	app := cli.NewApp()
	app.Usage = "A cli tool to enjoy uva oj!"
	app.UsageText = "uva [command]"
	app.Commands = []cli.Command{
		{
			Name:    "login",
			Aliases: []string{"l"},
			Usage:   "login to uva oj",
			Action: func(c *cli.Context) {
				var username string
				fmt.Print("Username: ")
				fmt.Scanln(&username)
				fmt.Print("Password: ")
				password, err := terminal.ReadPassword(0)
				if err != nil {
					panic(err)
				}
				login(username, string(password))
			},
		},
	}
	app.Run(os.Args)
}
