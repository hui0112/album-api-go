#!/usr/bin/env python3
"""
Performance Test for Shopping Cart API
=======================================
Runs exactly 150 operations (50 create, 50 add items, 50 get cart)
and records each operation's response time, status, and timestamp.

Usage:
  python3 load_test.py --url http://<ALB-DNS> --output mysql_test_results.json
  python3 load_test.py --url http://<ALB-DNS> --output dynamodb_test_results.json
"""

import argparse
import json
import time
from datetime import datetime, timezone

import requests


def run_test(base_url: str, output_file: str):
    results = []
    cart_ids = []

    # Warm up — send a few requests to initialize connection pools / cold starts
    print("Warming up...")
    for _ in range(3):
        try:
            requests.get(f"{base_url}/health", timeout=10)
        except Exception:
            pass
    time.sleep(1)

    # ---------------------------------------------------------------
    # Phase 1: Create 50 shopping carts
    # ---------------------------------------------------------------
    print("Phase 1: Creating 50 carts...")
    for i in range(50):
        start = time.time()
        try:
            resp = requests.post(
                f"{base_url}/shopping-carts",
                json={"customer_id": i + 1},
                timeout=10,
            )
            elapsed = (time.time() - start) * 1000  # Convert to milliseconds

            results.append({
                "operation": "create_cart",
                "response_time": round(elapsed, 2),
                "success": resp.status_code == 201,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            })

            if resp.status_code == 201:
                cart_id = resp.json().get("shopping_cart_id")
                cart_ids.append(cart_id)
            else:
                print(f"  Cart {i+1} failed: {resp.status_code} {resp.text}")
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "create_cart",
                "response_time": round(elapsed, 2),
                "success": False,
                "status_code": 0,
                "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            })
            print(f"  Cart {i+1} error: {e}")

    print(f"  Created {len(cart_ids)} carts successfully")

    # ---------------------------------------------------------------
    # Phase 2: Add items to each cart
    # ---------------------------------------------------------------
    print("Phase 2: Adding items to 50 carts...")
    for i, cart_id in enumerate(cart_ids[:50]):
        start = time.time()
        try:
            resp = requests.post(
                f"{base_url}/shopping-carts/{cart_id}/items",
                json={"product_id": (i % 10) + 1, "quantity": (i % 5) + 1},
                timeout=10,
            )
            elapsed = (time.time() - start) * 1000

            results.append({
                "operation": "add_items",
                "response_time": round(elapsed, 2),
                "success": resp.status_code == 204,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            })

            if resp.status_code != 204:
                print(f"  Add item to cart {cart_id} failed: {resp.status_code}")
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "add_items",
                "response_time": round(elapsed, 2),
                "success": False,
                "status_code": 0,
                "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            })
            print(f"  Add item error: {e}")

    # ---------------------------------------------------------------
    # Phase 3: Get each cart
    # ---------------------------------------------------------------
    print("Phase 3: Getting 50 carts...")
    for i, cart_id in enumerate(cart_ids[:50]):
        start = time.time()
        try:
            resp = requests.get(
                f"{base_url}/shopping-carts/{cart_id}",
                timeout=10,
            )
            elapsed = (time.time() - start) * 1000

            results.append({
                "operation": "get_cart",
                "response_time": round(elapsed, 2),
                "success": resp.status_code == 200,
                "status_code": resp.status_code,
                "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            })

            if resp.status_code != 200:
                print(f"  Get cart {cart_id} failed: {resp.status_code}")
        except Exception as e:
            elapsed = (time.time() - start) * 1000
            results.append({
                "operation": "get_cart",
                "response_time": round(elapsed, 2),
                "success": False,
                "status_code": 0,
                "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
            })
            print(f"  Get cart error: {e}")

    # ---------------------------------------------------------------
    # Save results
    # ---------------------------------------------------------------
    with open(output_file, "w") as f:
        json.dump(results, f, indent=2)

    # Print summary
    total = len(results)
    successes = sum(1 for r in results if r["success"])
    avg_time = sum(r["response_time"] for r in results) / total if total > 0 else 0

    print(f"\n{'='*50}")
    print(f"Results saved to: {output_file}")
    print(f"Total operations: {total}")
    print(f"Successes: {successes}/{total} ({successes/total*100:.1f}%)")
    print(f"Average response time: {avg_time:.2f} ms")

    # Per-operation breakdown
    for op in ["create_cart", "add_items", "get_cart"]:
        op_results = [r for r in results if r["operation"] == op]
        if op_results:
            times = [r["response_time"] for r in op_results]
            op_success = sum(1 for r in op_results if r["success"])
            print(f"  {op}: avg={sum(times)/len(times):.2f}ms, "
                  f"success={op_success}/{len(op_results)}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Shopping Cart API Load Test")
    parser.add_argument("--url", required=True, help="Base URL (e.g., http://hw8-alb-xxx.us-east-1.elb.amazonaws.com)")
    parser.add_argument("--output", required=True, help="Output JSON file (e.g., mysql_test_results.json)")
    args = parser.parse_args()

    # Remove trailing slash
    base_url = args.url.rstrip("/")

    print(f"Testing: {base_url}")
    print(f"Output:  {args.output}")
    print()

    run_test(base_url, args.output)
