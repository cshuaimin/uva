package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/crypto/ssh/terminal"
	"golang.org/x/net/publicsuffix"
)

const baseURL = "https://uva.onlinejudge.org"

var uvaURL, _ = url.Parse(baseURL)

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
				cprintf(red, 0, "%s\n", err)
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
	titleRegex := regexp.MustCompile("(\\d+)\u00A0-\u00A0(.+)")
	trueIDRegex := regexp.MustCompile(`.+problem=(\d+)`)
	getProblems := func() {
		defer func() {
			if err := recover(); err != nil {
				cprintf(red, 0, "%s\n", err)
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
					match := titleRegex.FindStringSubmatch(ele.Text())[1:]
					problem.ID, _ = strconv.Atoi(match[0])
					problem.Title = string(match[1])
					href, ok := ele.Attr("href")
					if !ok {
						panic("href not exists")
					}
					problem.TrueID, _ = strconv.Atoi(trueIDRegex.FindStringSubmatch(href)[1])
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

func crawlTestData(pid int) (input string, output string) {
	defer spin("Downloading test cases")()
	problemHomePage := fmt.Sprintf("https://www.udebug.com/UVa/%d", pid)
	doc, err := goquery.NewDocument(problemHomePage)
	if err != nil {
		panic(err)
	}
	sel := doc.Find("a.input_desc")
	// some problems has no input
	if sel.Length() != 0 {
		inputID, ok := sel.Attr("data-id")
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
		input = m["input_value"]
	}
	form := url.Values{}
	doc.Find("#udebug-custom-problem-view-input-output-form input").Each(func(i int, s *goquery.Selection) {
		form.Set(s.AttrOr("name", ""), s.AttrOr("value", ""))
	})
	if input != "" {
		form.Set("input_data", input)
	}
	resp, err := http.PostForm(problemHomePage, form)
	if err != nil {
		panic(err)
	}
	doc, err = goquery.NewDocumentFromResponse(resp)
	if err != nil {
		panic(err)
	}
	output = doc.Find("#edit-output-data").Text()
	return
}

type loginInfo struct {
	// Export these fields so that gob can dump them.
	Username string
	Cookies  []*http.Cookie
}

func login() (username string) {
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

	defer spin("Signing in uva.onlinejudge.org")()
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
	return
}
