package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
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
