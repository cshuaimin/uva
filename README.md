# uva-cli

A productive cli tool to enjoy [UVa Online Judge](https://uva.onlinejudge.org)!

- A very effficient way to fight questions.
- Test the code locally,
- or submit it to the UVa.

## Screenshot

[![asciicast](https://asciinema.org/a/4ootHvOrElVB52H050jDvlWGU.png)](https://asciinema.org/a/4ootHvOrElVB52H050jDvlWGU)

## Installation

### Install prebuilt packages

Download prebuilt binary from [releases page](https://github.com/cshuaimin/uva/releases)
Open/extract the archive.
Move uva to your path (/usr/loca/bin for example).

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
