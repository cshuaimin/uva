#! /usr/bin/python3

import os
import re
import time
import pickle
import requests
from tqdm import tqdm
from bs4 import BeautifulSoup
from argparse import ArgumentParser
from multiprocessing.dummy import Pool as ThreadPool

# your UVa ID
username = 'xx'
passwd = 'xx'

# some settings
login_cookie_file = os.path.expanduser('~/UVaOJ/login_cookie')
download_dir = os.path.expanduser('~/UVaOJ/downloads/')
true_problemids_file = os.path.expanduser('~/UVaOJ/true_problemids')

# change to your proxy
proxies = {
    'http': 'socks5://127.0.0.1:1080',
    'https': 'socks5://127.0.0.1:1080'
}


def volume_to_category(volume):
    if volume <= 9:
        return volume + 2
    elif 10 <= volume <= 12:
        return volume + 235
    elif 13 <= volume <= 15:
        return volume + 433
    elif volume == 16:
        return 825
    elif volume == 17:
        return 859


def get_true_problemid(problemid):
    def get_one_volume(volume):
        category = volume_to_category(volume)
        r = requests.get(
                'https://uva.onlinejudge.org/index.php?option=com_onlinejudge'
                '&Itemid=8&category=%s' % category,
                proxies=proxies)
        r.raise_for_status()
        return re.findall(
                r'<a href="index.php\?option=com_onlinejudge&amp;Itemid=8&amp;'
                r'category=\d+?&amp;page=show_problem&amp;problem=(\d+?)">.*?'
                r'</a>', r.text)
    if os.path.exists(true_problemids_file):
        with open(true_problemids_file, 'rb') as f:
            true_problemids = pickle.load(f)
    else:
        print('Getting true problem IDs...')
        pool = ThreadPool(17)   # total 17 volumes...
        nested = pool.map(get_one_volume, range(1, 18))
        pool.close()
        pool.join()
        true_problemids = [v for sub in nested for v in sub]
        with open(true_problemids_file, 'wb') as f:
            pickle.dump(true_problemids, f)
    return true_problemids[problemid - 100]


def download(url, file_name, chunk_size=16*1024):
    print('Connecting...', end='', flush=True)
    r = requests.get(url, stream=True, proxies=proxies)
    r.raise_for_status()
    print('\r' + ' ' * 20, end='', flush=True)
    with open(file_name, 'wb') as f, tqdm(
            desc='Downloading', leave=False,
            total=int(r.headers['Content-Length']),
            unit='B', unit_scale=True, unit_divisor=1024) as bar:
        for buf in r.iter_content(chunk_size):
            f.write(buf)
            bar.update(chunk_size)


def show_problem(problemid):
    file_name = download_dir + '%s.pdf' % problemid
    if not os.path.exists(file_name):
        download(
            'https://uva.onlinejudge.org/external/{}/p{}.pdf'
            .format(problemid // 100, problemid), file_name
        )
    os.system('evince {} 2>/dev/null'.format(file_name))


def login(username, passwd):
    s = requests.Session()
    s.headers['User-Agent'] = 'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.96 Safari/537.36'
    s.proxies = proxies
    if os.path.exists(login_cookie_file):
        print('Using saved login cookies...')
        with open(login_cookie_file, 'rb') as f:
            s.cookies = pickle.load(f)
        return s

    print('Logging you...')
    r = s.get('https://uva.onlinejudge.org/')
    r.raise_for_status()
    soup = BeautifulSoup(r.text, 'lxml')
    login_inputs = soup.form.find_all('input')
    login_data = {tag['name']: tag.get('value') for tag in login_inputs}
    login_data['username'] = username
    login_data['passwd'] = passwd
    r = s.post(
            'https://uva.onlinejudge.org/index.php?option=com_comprofiler'
            '&task=login', data=login_data)
    r.raise_for_status()
    with open(login_cookie_file, 'wb') as f:
        pickle.dump(s.cookies, f)
    return s


def submit(session, problemid, file):
    category = problemid // 100
    problemid = get_true_problemid(problemid)
    submit_url = 'https://uva.onlinejudge.org/index.php'
    submit_url += '?option=com_onlinejudge&Itemid=8&page=save_submission'
    submit_data = {'problemid': problemid, 'category': category, 'language': 3}
    print('Uploading...', end='', flush=True)
    with open(file) as f:
        r = session.post(
                submit_url, data=submit_data,
                files={'codeupl': f},
                # for speed
                allow_redirects=False)
    print()
    r.raise_for_status()
    return re.findall(
            r'&mosmsg=Submission\+received\+with\+ID\+(\d+)',
            r.headers['Location'])[0]


def get_result(session, submit_id):
    url = 'https://uva.onlinejudge.org/index.php'
    url += '?option=com_onlinejudge&Itemid=9'
    r = session.get(url)
    r.raise_for_status()
    soup = BeautifulSoup(r.text, 'lxml')
    for tr in soup.find(id='col3_content_wrapper').find_all('tr'):
        tds = tr.find_all('td')
        if tds[0].text == submit_id:
            return tds[3]
    return 'not found'


def main():
    ap = ArgumentParser(
            description="UVaClient - download UVaOJ's problem "
            "descriptin PDF and submit your code.")
    ap.add_argument('problemid', type=int, help="UVaOJ's problem ID")
    ap.add_argument(
            '-d', action='store_true',
            help='download and show the problem description')
    ap.add_argument('-s', help='submit a source file')
    args = ap.parse_args()

    if args.d:
        show_problem(args.problemid)
    else:
        s = login(username, passwd)
        submit_id = submit(s, args.problemid, args.s)

        result = get_result(s, submit_id)
        if 'In judge queue' in result.text:
            print('Judging...', end='', flush=True)
            while True:
                time.sleep(1)
                print('.', end='', flush=True)
                result = get_result(s, submit_id)
                if 'In judge queue' not in result.text:
                    print()
                    break

        print('Result: ' + result.text.strip())
        if result.a:
            print(
                'Follow this link for more info: '
                'https://uva.onlinejudge.org/' + result.a['href']
            )


if __name__ == '__main__':
    main()
