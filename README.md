> A productive cli tool to enjoy [UVa Online Judge](https://uva.onlinejudge.org)!

A very effficient way to fight questions:
- Print the problem description in terminal with a format like man(1).
- Compile and test the code locally, use test cases from udebug.com.
- Use a special diff algorithm to compare the output with the answer.
- Finally, you can submit the code to online judge and get result.

## Screenshot

[![asciicast](https://asciinema.org/a/hM9Qn8iS0ugrHCXrP3JkSIVSz.svg)](https://asciinema.org/a/hM9Qn8iS0ugrHCXrP3JkSIVSz)

## Installation

### Install prebuilt packages

1. Download prebuilt binary from [releases page](https://github.com/cshuaimin/uva/releases).
2. Open/extract the archive.
3. Move uva to your path (/usr/local/bin for example).

### Build from source

```sh
$ go get github.com/cshuaimin/uva
```

## Quick start

- Login with your UVa account:

  ```sh
  $ uva user -l
  ```

- Show description:

  ```sh
  $ uva show <ID>
  ```

- Run tests:

  ```sh
  $ uva test <file>
  ```

- Submit it!

  ```sh
  $ uva submit <file>
  ```
