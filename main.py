import hashlib
import time
import json
import requests
from bs4 import BeautifulSoup
import re

DEBUG = True
page = 1
url = f"https://www.list.am/category/60/{page}?n=1&price1=45000&price2=95000&_a39=2&_a11_1=4"
items_page_count = 90
excluded_keywords = ['Առնո Բաբաջանյան', 'Նորաշեն թաղամաս']
only_keywords = []
announcements = []
flaresolverr_url = "http://localhost:8191/v1"

def get_page_content(url):
    payload = {
        "cmd": "request.get",
        "url": url,
        "maxTimeout": 60000
    }
    response = requests.post(flaresolverr_url, json=payload)
    data = response.json()
    if 'solution' in data:
        return data['solution']['response']
    else:
        print("Failed to bypass Cloudflare")
        return None

class Announcement:
    def __init__(self, _link, _title, _price, _image_url, _region, _labels, _actual_floor,
                 _max_floor, _size, _rooms):
        self.link = f"https://list.am{_link}"
        self.title = _title
        self.price = _price
        self.image_url = _image_url
        self.labels = _labels
        self.actual_floor = _actual_floor
        self.max_floor = _max_floor
        self.size = _size
        self.rooms = _rooms

        self.sq_price = 99999999

    def __repr__(self):
        return f"Announcement(link={self.link}, title={self.title}, price={self.price}), sq_price={self.sq_price})"

    def calculate_sq_price(self):
        self.sq_price = self.price / self.size

page_content = get_page_content(url)
if page_content:
    soup = BeautifulSoup(page_content, 'html.parser')
    items = soup.select('div.dl div.gl a')
    last = False
    while True:
        print(f"page: {page}")
        if len(items) <= items_page_count:
            last = True
        if DEBUG:
            print(items)
        for item in items:
            try:
                exclude = False
                if DEBUG:
                    print(100*'-')
                number1 = 1
                number2 = 1
                actual_floor = 1
                max_floor = 1
                link = item.get('href')
                try:
                    price_text = item.select('div.p')[0].text
                    price = int(price_text.replace('$', '').replace(',', '').replace('€', ''))
                except Exception as e:
                    price = 9999999

                try:
                    labels = item.select('div.clabel')[0].text
                except Exception as e:
                    labels = ''
                title = item.select('div.l')[0].text
                for keyword in excluded_keywords:
                    if keyword in title:
                        exclude = True
                        break
                if only_keywords:
                    exclude = True
                    for keyword in only_keywords:
                        if keyword in title:
                            exclude = False
                if exclude:
                    continue
                img = item.select('img')[0].src
                at = item.select('div.at')[0].text
                at_list = at.split(',')
                region = at_list[0]
                try:
                    rooms = int(re.findall(r'\d+/\d+|\d+', at_list[1])[0])
                except Exception as e:
                    rooms = 0
                try:
                    size = int(re.findall(r'\d+/\d+|\d+', at_list[2])[0])
                except Exception as e:
                    size = 1
                try:
                    match = re.search(r'(\d+)/(\d+) հարկ', at_list[3])
                except Exception as e:
                    match = None
                if match:
                    actual_floor = match.group(1)
                    max_floor = match.group(2)
                else:
                    print("Pattern not found")
                if DEBUG:
                    print(f"Price: {price}")
                    print(f"title: {title}")
                    print(f"region: {region}")
                    print(f"rooms: {rooms}")
                    print(f"size: {size}")
                    print("actual_floor:", number1)
                    print("max_floor:", number2)
                    print(f"link: https://list.am/{link}")
                ann = Announcement(
                    _link=link,
                    _title=title,
                    _price=price,
                    _image_url=img,
                    _region=region,
                    _labels=labels,
                    _actual_floor=actual_floor,
                    _max_floor=max_floor,
                    _size=size,
                    _rooms=rooms,
                )
                ann.calculate_sq_price()
                announcements.append(ann)
            except Exception as e:
                print("Could not get item attributes")
                print(e)
        if last:
            break
        else:
            page += 1
            url = f"https://www.list.am/category/60/{page}?n=1&price1=45000&price2=90000&_a39=2&_a11_1=4"
            page_content = get_page_content(url)
            if not page_content:
                break
            soup = BeautifulSoup(page_content, 'html.parser')
            items = soup.select('div.dl div.gl a')
        time.sleep(1)

data_diff = {}
data_loaded = {}
try:
    with open('data.json', 'r') as json_file:
        data_loaded = json.load(json_file)
except FileNotFoundError:
    pass

data = {}
sorted_announcements = sorted(announcements, key=lambda x: x.sq_price)
for announcement in sorted_announcements:
    item = {
        "sq_price": announcement.sq_price,
        "title": announcement.title,
        "price": announcement.price,
        "link": announcement.link,
    }
    md5_hash = hashlib.md5()
    md5_hash.update(f"{item['link']}{item['price']}".encode('utf-8'))
    md5_hex = md5_hash.hexdigest()
    data[md5_hex] = item

    if md5_hex not in data_loaded:
        data_diff[md5_hex] = item
        print(announcement)

print(f"There are {len(data_diff)} different items in the list.")

with open('data.json', 'w') as json_file:
    json.dump(data, json_file, indent=4)
print("Data written to data.json")
