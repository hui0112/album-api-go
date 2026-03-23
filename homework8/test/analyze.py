#!/usr/bin/env python3
"""
STEP III Analysis Script
========================
Reads mysql_test_results.json and dynamodb_test_results.json,
merges them into combined_results.json, and prints comparison tables.

Usage:
  python3 analyze.py
  (run from the homework8/ directory)
"""

import json
import os
import statistics


def load_results(filepath: str) -> list:
    """Load test results from a JSON file."""
    if not os.path.exists(filepath):
        print(f"WARNING: {filepath} not found!")
        return []
    with open(filepath) as f:
        return json.load(f)


def percentile(data: list, p: int) -> float:
    """Calculate the p-th percentile of a list of numbers."""
    if not data:
        return 0.0
    sorted_data = sorted(data)
    k = (len(sorted_data) - 1) * p / 100
    f = int(k)
    c = f + 1
    if c >= len(sorted_data):
        return sorted_data[f]
    return sorted_data[f] + (k - f) * (sorted_data[c] - sorted_data[f])


def analyze_results(results: list, label: str) -> dict:
    """Analyze a set of test results and return stats."""
    if not results:
        return {}

    times = [r["response_time"] for r in results]
    successes = sum(1 for r in results if r["success"])

    stats = {
        "label": label,
        "total": len(results),
        "successes": successes,
        "success_rate": round(successes / len(results) * 100, 2),
        "avg": round(statistics.mean(times), 2),
        "p50": round(percentile(times, 50), 2),
        "p95": round(percentile(times, 95), 2),
        "p99": round(percentile(times, 99), 2),
    }

    # Per-operation breakdown
    for op in ["create_cart", "add_items", "get_cart"]:
        op_times = [r["response_time"] for r in results if r["operation"] == op]
        if op_times:
            stats[f"{op}_avg"] = round(statistics.mean(op_times), 2)
        else:
            stats[f"{op}_avg"] = 0

    return stats


def print_comparison_table(mysql_stats: dict, dynamo_stats: dict):
    """Print the comparison tables required by STEP III."""

    print("\n" + "=" * 70)
    print("STEP III Part 1: Performance Comparison Table")
    print("=" * 70)
    print(f"Data Source: combined_results.json")
    print()

    # Overall comparison
    header = f"{'Metric':<28} | {'MySQL':>10} | {'DynamoDB':>10} | {'Winner':>10} | {'Margin':>10}"
    print(header)
    print("-" * len(header))

    metrics = [
        ("Avg Response Time (ms)", "avg"),
        ("P50 Response Time (ms)", "p50"),
        ("P95 Response Time (ms)", "p95"),
        ("P99 Response Time (ms)", "p99"),
        ("Success Rate (%)", "success_rate"),
    ]

    for label, key in metrics:
        m_val = mysql_stats.get(key, 0)
        d_val = dynamo_stats.get(key, 0)

        if key == "success_rate":
            winner = "MySQL" if m_val >= d_val else "DynamoDB"
            margin = f"{abs(m_val - d_val):.2f}%"
        else:
            winner = "MySQL" if m_val <= d_val else "DynamoDB"
            margin = f"{abs(m_val - d_val):.2f}ms"

        print(f"{label:<28} | {m_val:>10} | {d_val:>10} | {winner:>10} | {margin:>10}")

    print(f"{'Total Operations':<28} | {'150':>10} | {'150':>10} |{'':>11} |{'':>11}")

    # Operation-specific breakdown
    print()
    print("Operation-Specific Breakdown:")
    header2 = f"{'Operation':<15} | {'MySQL Avg (ms)':>15} | {'DynamoDB Avg (ms)':>18} | {'Faster By':>12}"
    print(header2)
    print("-" * len(header2))

    for op, op_label in [("create_cart", "CREATE_CART"), ("add_items", "ADD_ITEMS"), ("get_cart", "GET_CART")]:
        m_val = mysql_stats.get(f"{op}_avg", 0)
        d_val = dynamo_stats.get(f"{op}_avg", 0)
        diff = abs(m_val - d_val)
        faster = "MySQL" if m_val <= d_val else "DynamoDB"
        print(f"{op_label:<15} | {m_val:>15} | {d_val:>18} | {faster} {diff:.2f}ms")


def main():
    # Load both result files
    base_dir = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    mysql_path = os.path.join(base_dir, "mysql_test_results.json")
    dynamo_path = os.path.join(base_dir, "dynamodb_test_results.json")
    combined_path = os.path.join(base_dir, "combined_results.json")

    mysql_results = load_results(mysql_path)
    dynamo_results = load_results(dynamo_path)

    if not mysql_results or not dynamo_results:
        print("ERROR: Both mysql_test_results.json and dynamodb_test_results.json are required!")
        print(f"  Checked: {mysql_path}")
        print(f"  Checked: {dynamo_path}")
        return

    # Verify data consistency
    print("Data Verification:")
    for name, results in [("MySQL", mysql_results), ("DynamoDB", dynamo_results)]:
        total = len(results)
        creates = sum(1 for r in results if r["operation"] == "create_cart")
        adds = sum(1 for r in results if r["operation"] == "add_items")
        gets = sum(1 for r in results if r["operation"] == "get_cart")
        print(f"  {name}: {total} total ({creates} create, {adds} add, {gets} get)")

    # Create combined results
    combined = {
        "mysql": mysql_results,
        "dynamodb": dynamo_results,
        "metadata": {
            "mysql_total": len(mysql_results),
            "dynamodb_total": len(dynamo_results),
            "generated_at": "auto",
        }
    }

    with open(combined_path, "w") as f:
        json.dump(combined, f, indent=2)
    print(f"\nSaved: {combined_path}")

    # Analyze
    mysql_stats = analyze_results(mysql_results, "MySQL")
    dynamo_stats = analyze_results(dynamo_results, "DynamoDB")

    # Print comparison
    print_comparison_table(mysql_stats, dynamo_stats)


if __name__ == "__main__":
    main()
