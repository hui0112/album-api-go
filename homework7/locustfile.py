"""
Locust Load Test for Homework 7 - Order Processing System

HOW TO USE:
-----------
1. Install Locust:    pip install locust

2. Test SYNC endpoint (Phase 1):
   Normal load:  locust --tags sync -u 5 -r 1 -t 30s --headless -H http://YOUR-ALB-DNS
   Flash sale:   locust --tags sync -u 20 -r 10 -t 60s --headless -H http://YOUR-ALB-DNS

3. Test ASYNC endpoint (Phase 3):
   Flash sale:   locust --tags async -u 20 -r 10 -t 60s --headless -H http://YOUR-ALB-DNS

4. With the Web UI (for real-time graphs):
   locust -H http://YOUR-ALB-DNS
   Then open http://localhost:8089 in your browser.

PARAMETERS EXPLAINED:
- -u 5:   5 concurrent users (simulated customers)
- -r 1:   Spawn 1 new user per second until target reached
- -r 10:  Spawn 10 new users per second (flash sale ramp-up)
- -t 30s: Run for 30 seconds then stop
- --tags: Only run tasks with matching tag (sync OR async)
- -H:     The base URL (your ALB DNS)
"""

import json
import random
import time
from locust import HttpUser, task, between, tag


class OrderUser(HttpUser):
    """
    Simulates a customer placing orders.

    wait_time = between(0.1, 0.5):
        After each request, the user waits a random 100-500ms before the next.
        This simulates realistic "think time" between orders.
        (The homework spec says: "User wait time: random 100-500ms between requests")
    """
    wait_time = between(0.1, 0.5)

    def _make_order(self):
        """
        Generate a random order payload.
        Each order has a random customer ID and 1-3 random items.
        """
        items = []
        for _ in range(random.randint(1, 3)):
            items.append({
                "product_id": random.randint(1, 1000),
                "name": f"Product-{random.randint(1, 100)}",
                "quantity": random.randint(1, 5),
                "price": round(random.uniform(9.99, 99.99), 2),
            })

        return {
            "customer_id": random.randint(1, 10000),
            "items": items,
        }

    @tag("sync")
    @task
    def place_sync_order(self):
        """
        Test the synchronous endpoint (Phase 1).

        Expected behavior:
        - Normal load (5 users): Most requests succeed in ~3 seconds
        - Flash sale (20 users): Requests queue up, timeouts increase,
          some customers wait 10+ seconds or get errors
        """
        order = self._make_order()
        self.client.post(
            "/orders/sync",
            json=order,
            name="/orders/sync",  # Group all requests under this name in stats
        )

    @tag("async")
    @task
    def place_async_order(self):
        """
        Test the asynchronous endpoint (Phase 3).

        Expected behavior:
        - All requests return 202 Accepted in <100ms
        - 100% acceptance rate even under flash sale load
        - Orders are queued in SQS for background processing
        """
        order = self._make_order()
        self.client.post(
            "/orders/async",
            json=order,
            name="/orders/async",
        )
