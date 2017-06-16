# UVaClient
UVaOJ命令行提交代码
## Usage
```zsh
➜  ./UVaClient.py --help
usage: UVaClient.py [-h] [-d] [-s S] problemid

UVaClient - download UVaOJ's problem descriptin PDF and submit your code.

positional arguments:
  problemid   UVaOJ's problem ID

optional arguments:
  -h, --help  show this help message and exit
  -d          download and show the problem description
  -s S        submit a source file
➜  
➜  # display problem #1368
➜  ./UVaClient.py -d 1368
Downloading /home/csm/UVaOJ/downloads/1368.pdf...
# the GUI PDF reader evince started, you can read problem description and then write your code
➜  
➜  # now submit t.c to problem #1368
➜  ./UVaClient.py -s t.c 1368
Logging you...
Getting true problem IDs...
Uploading...
Judging....
Result: Accepted
➜  
➜  echo 'hello' >> t.c
➜  ~ ./UVaClient.py -s t.c 1586
Using saved login cookies...
Uploading...
Result: Compilation error
Follow this link for more info: https://uva.onlinejudge.org/index.php?option=com_onlinejudge&Itemid=9&page=show_compilationerror&submission=19530928
➜  # happy coding!
```
## Requirements
```zsh
$ pip(3) install requests
$ pip(3) install 'requests[socks]'
$ pip(3) install bs4
```
