package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	humanize "github.com/dustin/go-humanize"
	"github.com/urfave/cli"
)

func user(c *cli.Context) {
	if c.Bool("l") {
		login()
	} else if c.Bool("L") {
		if err := os.Remove(loginInfoFile); err != nil {
			panic(err)
		}
	} else {
		fmt.Println("You are now logged in as", colored(loadLoginInfo().Username, yellow, bold))
	}
}

func printPdf(file string, info problemInfo) {
	pdf, err := exec.Command("pdftotext", file, "-").Output()
	if err != nil {
		panic(err)
	}
	description := string(pdf)
	title := fmt.Sprintf("%d - %s", info.ID, info.Title)
	padding := strings.Repeat(" ", (108-len(title))/2)
	cprintf(white, 1, "%s%s\n\n", padding, title)

	const indent = "       "
	cprintf(white, 1, "Statistics\n")
	fmt.Printf(indent+"* Rate: %.1f %%\n", info.Percentage)
	accepted := humanize.Bytes(uint64(float32(info.TotalSubmissions) * info.Percentage / 100))
	fmt.Printf(indent+"* Total Accepted: %s\n", accepted[:len(accepted)-1])
	submissions := humanize.Bytes(uint64(info.TotalSubmissions))
	fmt.Printf(indent+"* Total Submissions: %s\n\n", submissions[:len(submissions)-1])

	cprintf(white, 1, "Description\n")
	// indentation
	description = strings.Replace(description, "\n", "\n"+indent, -1)
	for _, s := range []string{"Input", "Output", "Sample Input", "Sample Output"} {
		description = strings.Replace(description, indent+s, colored(s, white, bold), 1)
	}
	description = indent + strings.TrimSpace(description)
	fmt.Println(description)
}

func show(c *cli.Context) {
	if c.NArg() == 0 {
		panic("problem id required")
	}
	pid, err := strconv.Atoi(c.Args().First())
	if err != nil {
		panic(err)
	}
	info := getProblemInfo(pid)
	pdfFile := pdfPath + info.getFileName("pdf")
	if !exists(pdfFile) {
		download(fmt.Sprintf("%s/external/%d/p%d.pdf", baseURL, pid/100, pid), pdfFile, "Downloading "+info.Title)
	}

	if c.Bool("g") {
		if err := exec.Command("evince", pdfFile).Run(); err != nil {
			panic(err)
		}
	} else {
		printPdf(pdfFile, info)
	}
}

func touch(c *cli.Context) {
	if c.NArg() == 0 {
		panic("problem ID required")
	}
	pid, err := strconv.Atoi(c.Args().First())
	if err != nil {
		panic(err)
	}
	name := getProblemInfo(pid).getFileName(c.String("lang"))
	f, err := os.Create(name)
	if err != nil {
		panic(err)
	}
	f.Close()
	fmt.Printf("Created %s\n", colored(name, yellow, underline))
}

func submit(problemID int, file string, lang int) string {
	category := problemID / 100
	info := getProblemInfo(problemID)
	problemID = info.TrueID
	form := url.Values{
		"problemid": {strconv.Itoa(problemID)},
		"category":  {strconv.Itoa(category)},
		"language":  {strconv.Itoa(lang)},
	}
	code, err := ioutil.ReadFile(file)
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
	sidRegex := regexp.MustCompile(`Submission\+received\+with\+ID\+(\d+)`)
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

func submitAndShowResult(c *cli.Context) {
	if c.NArg() == 0 {
		panic("filename required")
	}
	file := c.Args().First()
	pid, _, ext := parseFilename(file)
	var lang int
	switch ext {
	case "c":
		lang = ansic
	case "java":
		lang = java
	case "cc", "cpp":
		lang = cpp
	case "pas":
		lang = pascal
	case "py":
		lang = python3
	}
	sid := submit(pid, file, lang)
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
		cprintf(cyan, bold, "%s Accepted (%ss)\n", yes, runTime)
	} else {
		cprintf(red, bold, "%s %s\n", no, result)
	}
}

func testProgram(c *cli.Context) {
	if c.NArg() == 0 {
		panic("filename required")
	}
	if c.String("i") == "" && c.String("a") != "" {
		panic("flag -a must be used with -i")
	}
	file := c.Args().First()
	pid, _, ext := parseFilename(file)

	// compile source code
	loadConfig()
	compile := renderCmd(config.Test[ext].Compile, file)
	// for non-script languages
	if compile != nil {
		stop := spin("Compiling")
		out, err := compile.CombinedOutput()
		stop()
		failed := false
		if err != nil {
			if _, ok := err.(*exec.ExitError); ok {
				// a non-zero exit code means compilation failed
				failed = true
			} else {
				panic(err)
			}
		}
		if len(out) != 0 {
			if failed {
				cprintf(red, bold, no+" Compilation Error:\n\n")
				fmt.Print(string(out))
				os.Exit(1)
			} else {
				cprintf(magenta, bold, no+" Compilation Warning:\n\n")
				fmt.Print(string(out))
			}
		}
	}

	run := renderCmd(config.Test[ext].Run, file)
	var answer string
	if inputFile := c.String("i"); inputFile == "" {
		// get test case from udebug.com
		var input string
		input, answer = getTestData(pid)
		run.Stdin = strings.NewReader(input)
	} else {
		f, err := os.Open(inputFile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		run.Stdin = f
	}

	stop := spin("Running tests")
	start := time.Now()
	output, err := run.Output()
	runTime := time.Since(start)
	stop()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			// Print the output generated before the crash.
			fmt.Printf("%s\n\n", output)
			if status, ok := ee.Sys().(syscall.WaitStatus); ok {
				cprintf(red, bold, no+" Program exited with code %d\n\n", status.ExitStatus())
			} else {
				cprintf(red, bold, no+" Program exited with non-zero code\n\n")
			}
			fmt.Println(string(ee.Stderr))
			os.Exit(1)
		} else {
			panic(err)
		}
	}

	if c.String("i") != "" {
		if answerFile := c.String("a"); answerFile == "" {
			// If the input is provided but there is no answer, we do not compare.
			fmt.Println(string(output))
			return
		} else {
			data, err := ioutil.ReadFile(answerFile)
			if err != nil {
				panic(err)
			}
			answer = string(data)
		}
	}
	diff, same := wordDiff(answer, string(output), yes+" Answer", no+" Output")
	if same {
		cprintf(cyan, bold, yes+" Accepted (%.3fs)\n", float32(runTime)/float32(time.Second))
	} else {
		cprintf(red, bold, no+" Wrong answer\n\n")
		fmt.Print(diff)
	}
}

func dump(c *cli.Context) {
	if c.NArg() == 0 {
		panic("filename required")
	}
	file := c.Args().First()
	pid, _, _ := parseFilename(file)
	input, answer := getTestData(pid)
	if err := ioutil.WriteFile(c.String("i"), []byte(input), 0666); err != nil {
		panic(err)
	}
	if err := ioutil.WriteFile(c.String("a"), []byte(answer), 0666); err != nil {
		panic(err)
	}
	fmt.Printf("Dumped to %s and %s\n", colored(c.String("i"), yellow, underline), colored(c.String("a"), yellow, underline))
}
