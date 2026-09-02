import time
from locust import HttpUser, between, task, events, tag
from random import randint, choice, shuffle, gauss, choices
from pyquery import PyQuery
import itertools


PROJECTS = {
    "project_837286708028879339": ["label_0", "label_1", "label_2", "label_3", "label_4"],
}

def num_pages(pq):
    els = pq("main > nav > a")
    if len(els) == 0:
        return None

    return int(els[-2].text)

class PatchworkUser(HttpUser):
    wait_time = between(1, 5)

    @tag('basic')
    @tag('pages_crawl')
    @task
    def pages_crawl(self):
        for project in list(PROJECTS.keys()):
            url = f"project/{project}/list/?"

            result = self.client.get(url)

            np = num_pages(PyQuery(result.content))
            print("pages", np)

            if np is not None:
                for page_number in range(1, min(np+1, 50)):
                    print("page url", url + f"page={page_number}")

                    self.client.get(url + f"page={page_number}", name=f"page")

            print('pages finished')


        for project in PROJECTS.keys():
            labels_org = PROJECTS[project].copy()

            for num_labels in range(1, 3):
                for labels in list(itertools.combinations(labels_org[:5], num_labels)):
                    labels_search = "+".join(labels)

                    url = f"project/{project}/list/"
                    url += f"?labels={labels_search}"

                    print(url)
                    result = self.client.get(url, name=f"labels_{len(labels)}")

                    if result.content is None:
                        print("result is none?", url)
                        return

                    np = num_pages(PyQuery(result.content))
                    print("pages", np)

                    if np is not None:
                        for page_number in range(randint(2, min(np, 6))):
                            print("page url", url + f"&page={page_number}")

                            self.client.get(url + f"&page={page_number}", name=f"labels_{len(labels)}")


