from locust import FastHttpUser, task, between
import random

# Search terms that match our generated product data
SEARCH_TERMS = [
    "Electronics", "Books", "Home", "Sports", "Clothing",
    "Toys", "Food", "Health",
    "Alpha", "Beta", "Gamma", "Delta", "Epsilon",
    "Zeta", "Eta", "Theta",
    "Product",
]


class ProductSearchUser(FastHttpUser):
    # Minimal wait time to simulate high load
    wait_time = between(0.01, 0.05)

    @task
    def search_product(self):
        query = random.choice(SEARCH_TERMS)
        self.client.get(
            f"/products/search?q={query}",
            name="/products/search",
        )
