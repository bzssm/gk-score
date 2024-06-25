import json
import os.path
import queue
import sys
import threading
import time
from itertools import product

from loguru import logger

import requests

SCHOOL_INDEX_URL = "https://static-data.gaokao.cn/www/2.0/school/name.json"
# 专业分数线，args: school_id, year, province_id
SCHOOL_SPECIAL_SCORE_URL = "https://static-data.gaokao.cn/www/2.0/schoolspecialscore/{}/{}/{}.json"
YEAR_LIST = [2018, 2019, 2020, 2021, 2022, 2023]
SCHOOL_SPECIAL_SCORE_DIR = "school_special_score"
os.makedirs(SCHOOL_SPECIAL_SCORE_DIR, exist_ok=True)

# 1. 获取所有学校信息
if os.path.exists('school_index.json'):
    school_index = json.loads(open('school_index.json').read())
else:
    resp = requests.get(SCHOOL_INDEX_URL)
    if resp.status_code != 200:
        logger.error(f"Failed to fetch school index, status code: {resp.status_code}")
        sys.exit(1)
    school_index = json.loads(resp.text)['data']
    logger.debug(f"school index: {school_index}")
    # save school index to file
    with open("school_index.json", 'w') as f:
        f.write(json.dumps(sorted(school_index, key=lambda x: int(x['school_id'])), sort_keys=True, indent=2, ensure_ascii=False))
logger.info(f"school index loaded: {len(school_index)}")

# 2. 并行获取学校详细信息
# data字段：1_7_0：理科一本，2_7_0 文科一本，1_8_0 理科二本，2_8_0 文科二本， 1_10_0 理科专科， 2_10_0 文科专科
# 三本应该是隐藏字段了
# 这里指考虑170 和 270
school_special_score_url_queue = queue.Queue()
for school_year in product([x['school_id'] for x in school_index], YEAR_LIST):
    school_special_score_url_queue.put(school_year)

# 并发获取
def get_school_special_score():
    while not school_special_score_url_queue.empty():
        school_year = school_special_score_url_queue.get()
        url = SCHOOL_SPECIAL_SCORE_URL.format(*school_year, 45)

        for retry in range(5):
            try:
                resp = requests.get(url)
            except Exception as e:
                time.sleep(30)  # if exception happens
                logger.error(e)
                if retry == 4:
                    logger.error(f"Too many reties when fetch school special details, url: {url}")
                continue

            if resp.status_code != 200:
                logger.error(f"Fetch school detail failed, url: {url} status code: {resp.status_code}")
                break
            data = json.loads(resp.text)['data']
            # save school special score to file
            # 格式：school_id_year.json
            with open(f"{SCHOOL_SPECIAL_SCORE_DIR}/{school_year[0]}_{school_year[1]}.json", 'w') as f:
                f.write(json.dumps(data, sort_keys=True, indent=2, ensure_ascii=False))
            logger.info(f"school special score saved: {url}")
            break # success, break


threads = []
for _ in range(50):
    t = threading.Thread(target=get_school_special_score)
    t.start()
    threads.append(t)
for t in threads:
    t.join()

logger.info("All done!")


