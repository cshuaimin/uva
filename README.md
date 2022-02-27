## A productive cli tool to enjoy [UVa Online Judge](https://onlinejudge.org)!

A very effficient way to fight questions:
- Print the problem description in terminal with a format like man(1).
- Compile and test the code locally, using test cases from udebug.com.
- Use a special diff algorithm to compare the output with the answer.
- Finally, you can submit the code to online judge and get result.

## Screenshot

[![asciicast](https://asciinema.org/a/hM9Qn8iS0ugrHCXrP3JkSIVSz.svg)](https://asciinema.org/a/hM9Qn8iS0ugrHCXrP3JkSIVSz)

## Installation

### Install prebuilt packages

1. Download prebuilt binary from [releases page](https://github.com/cshuaimin/uva/releases).
2. Open/extract the archive.
3. Move uva to your path (/usr/local/bin for example).
4. (macOS) Install `pdftotext` cli: `brew install poppler`

### Build from source

```sh
$ go install github.com/cshuaimin/uva@latest
```

## Usage
```console
$ uva -h
NAME:
   uva - A cli tool to enjoy uva oj!

USAGE:                                                                                                                                 
   uva [command]

VERSION:
   0.3.0

COMMANDS:
     user     manage account
     show     show problem by id
     touch    create source file
     submit   submit code
     test     test code locally
     dump     dump test cases to files
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```

```console
$ uva test -h                      
NAME:
   uva test - test code locally

USAGE:
   uva test {id}.{name}.{ext}}
   uva test 10041.happy.cpp

OPTIONS:
   -i value  input file
   -a value  answer file
   -b        compare each line of output with the answer byte-by-byte
```
