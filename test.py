import time
import requests
import csv

from requests.models import HTTPError

def sqlQuery(sql: str) -> requests.Response:
    url = "http://localhost:4400/testquery"
    form_data = {"query": sql}
    r = requests.post(url, data=form_data)
    return r

def gnomeQuery(question: str) -> requests.Response:
    url = "http://localhost:4400/testgnome"
    form_data = {"filter-request": question}
    r = requests.post(url, data=form_data)
    return r


# read benchmark questions
questions = []
with open("test_questions.csv", "r") as testfile:
    questionReader = csv.reader(testfile)
    for row in questionReader:
        questions.append(row)

# remove headers
questions = questions[1:]

correctCount = 0
for i in range(len(questions)):
    time.sleep(1)
    pair = questions[i]
    # if this fails, then -1 points
    try:
        rQuestion = gnomeQuery(pair[0])
    except HTTPError as e:
        print(f"Gnome: {e.response.status_code} {e.response.text}")
        continue

    time.sleep(1)

    # if this fails, there is a bug in the testing system
    try:
        rSql = sqlQuery(pair[1])
        rSql.raise_for_status()
    except HTTPError as e:
        print(f"Sql {pair[0]}: {e.response.status_code} {e.response.text}")
        continue

    if rQuestion.json() != rSql.json():
        print(f"Failed on {i} {pair[0]}")
        continue
    else:
        print(f"Success on {i}")

    correctCount += 1

print(f"Score: {correctCount} / {len(questions)}")
