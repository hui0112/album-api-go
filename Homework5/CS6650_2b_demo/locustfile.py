from locust import HttpUser, FastHttpUser, task, between
import random
import string

# ---------- Helper ----------
# Generates a random SKU string like "SKU-aBcDe" for each POST request
# so every request sends slightly different data (more realistic)
def random_sku():
    return "SKU-" + "".join(random.choices(string.ascii_letters, k=5))


# ---------- HttpUser version ----------
# HttpUser uses Python's `requests` library under the hood.
# It's simpler and more compatible, but slower because each request
# creates overhead from the requests library.
class ProductHttpUser(HttpUser):
    # wait_time = how long a simulated user waits between requests
    # between(1, 3) means randomly wait 1-3 seconds (simulates real user think time)
    wait_time = between(1, 3)

    # @task(weight) — the weight controls how often this task runs relative to others.
    # weight=3 for GET vs weight=1 for POST means ~75% reads, ~25% writes.
    # This simulates a real e-commerce site where people browse more than they create products.
    @task(3)
    def get_product(self):
        # Pick a random product ID between 1-100
        product_id = random.randint(1, 100)
        # Locust expects the response — a 404 (product doesn't exist) is still a valid test.
        # name= groups all /products/X requests under one label in Locust's stats
        # (otherwise each ID gets its own row which is messy)
        self.client.get(f"/products/{product_id}", name="/products/[id]")

    @task(1)
    def create_product(self):
        product_id = random.randint(1, 100)
        self.client.post(
            f"/products/{product_id}/details",
            json={
                "sku": random_sku(),
                "manufacturer": "TestCorp",
                "category_id": random.randint(1, 20),
                "weight": random.randint(100, 5000),
                "some_other_id": random.randint(1, 50),
            },
            # Same grouping trick — all POSTs appear as one row in stats
            name="/products/[id]/details",
        )


# ---------- FastHttpUser version ----------
# FastHttpUser uses `geventhttpclient` instead of `requests`.
# It's faster and uses less CPU, so you can simulate more users per machine.
# The tradeoff: slightly less compatible with some HTTP edge cases,
# but for simple JSON APIs like ours, it works perfectly.
class ProductFastHttpUser(FastHttpUser):
    wait_time = between(1, 3)

    @task(3)
    def get_product(self):
        product_id = random.randint(1, 100)
        self.client.get(f"/products/{product_id}", name="/products/[id]")

    @task(1)
    def create_product(self):
        product_id = random.randint(1, 100)
        self.client.post(
            f"/products/{product_id}/details",
            json={
                "sku": random_sku(),
                "manufacturer": "TestCorp",
                "category_id": random.randint(1, 20),
                "weight": random.randint(100, 5000),
                "some_other_id": random.randint(1, 50),
            },
            name="/products/[id]/details",
        )
