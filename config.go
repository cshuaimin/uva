package main

import (
	"os"
)

var (
	dataPath         = os.Getenv("HOME") + "/.local/share/uva-cli/"
	pdfPath          = dataPath + "pdf/"
	testDataPath     = dataPath + "test-data"
	loginInfoFile    = dataPath + "login-info.gob"
	problemsInfoFile = dataPath + "problems-info.gob"
)

const baseURL = "https://uva.onlinejudge.org"
