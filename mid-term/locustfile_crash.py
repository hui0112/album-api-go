from locust import HttpUser, task, between

class CrashTestUser(HttpUser):
    wait_time = between(0.5, 1.5)

    @task(7)
    def read_albums(self):
        """70% of traffic: normal read operation"""
        self.client.get("/albums")

    @task(3)
    def create_order(self):
        """30% of traffic: order creation (calls downstream service)"""
        self.client.post("/orders", json={
            "id": "order-1",
            "item": "Blue Train Album",
            "quantity": 1,
            "price": 56.99,
        })
