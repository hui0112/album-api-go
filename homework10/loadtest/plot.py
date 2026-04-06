"""
Load test result plotter for HW10.

Usage:
    python plot.py results_dir/

    results_dir/ should contain CSV files named like:
        lf-w5r1-1pct.csv, lf-w1r5-10pct.csv, ll-50pct.csv, etc.

    Outputs PNG graphs into the same directory.
"""

import sys
import os
import glob
import pandas as pd
import matplotlib.pyplot as plt
import numpy as np


def load_results(results_dir):
    """Load all CSV files from the results directory."""
    files = sorted(glob.glob(os.path.join(results_dir, "*.csv")))
    if not files:
        print(f"No CSV files found in {results_dir}")
        sys.exit(1)

    datasets = {}
    for f in files:
        name = os.path.splitext(os.path.basename(f))[0]
        df = pd.read_csv(f)
        datasets[name] = df
        print(f"Loaded {name}: {len(df)} rows")
    return datasets


def plot_latency_distribution(datasets, results_dir):
    """
    Plot read and write latency distributions for each dataset.
    Uses histogram to show long-tail effects.
    """
    for name, df in datasets.items():
        fig, axes = plt.subplots(1, 2, figsize=(14, 5))
        fig.suptitle(f"Latency Distribution: {name}", fontsize=14)

        for ax, op in zip(axes, ["read", "write"]):
            subset = df[df["operation"] == op]["latency_ms"]
            if len(subset) == 0:
                ax.set_title(f"{op.capitalize()} (no data)")
                continue

            ax.hist(subset, bins=50, edgecolor="black", alpha=0.7,
                    color="steelblue" if op == "read" else "coral")
            ax.set_xlabel("Latency (ms)")
            ax.set_ylabel("Count")
            ax.set_title(f"{op.capitalize()} Latency (n={len(subset)}, "
                         f"avg={subset.mean():.1f}ms, p99={subset.quantile(0.99):.1f}ms)")
            ax.axvline(subset.median(), color="red", linestyle="--",
                       label=f"median={subset.median():.1f}ms")
            ax.legend()

        plt.tight_layout()
        out = os.path.join(results_dir, f"{name}-latency.png")
        plt.savefig(out, dpi=150)
        plt.close()
        print(f"Saved {out}")


def plot_latency_comparison(datasets, results_dir):
    """
    Box plot comparing read/write latencies across all configurations.
    """
    read_data, write_data = [], []
    read_labels, write_labels = [], []

    for name, df in datasets.items():
        reads = df[df["operation"] == "read"]["latency_ms"]
        writes = df[df["operation"] == "write"]["latency_ms"]
        if len(reads) > 0:
            read_data.append(reads.values)
            read_labels.append(name)
        if len(writes) > 0:
            write_data.append(writes.values)
            write_labels.append(name)

    for data, labels, op in [(read_data, read_labels, "Read"),
                              (write_data, write_labels, "Write")]:
        if not data:
            continue
        fig, ax = plt.subplots(figsize=(max(10, len(labels) * 1.5), 6))
        ax.boxplot(data, labels=labels, showfliers=False)
        ax.set_ylabel("Latency (ms)")
        ax.set_title(f"{op} Latency Comparison Across Configurations")
        plt.xticks(rotation=45, ha="right")
        plt.tight_layout()
        out = os.path.join(results_dir, f"comparison-{op.lower()}-latency.png")
        plt.savefig(out, dpi=150)
        plt.close()
        print(f"Saved {out}")


def plot_rw_time_interval(datasets, results_dir):
    """
    Plot distribution of time intervals between reading and writing the same key.
    Only considers reads that have a prior write (time_since_last_write_ms > 0).
    """
    for name, df in datasets.items():
        reads = df[(df["operation"] == "read") & (df["time_since_last_write_ms"] > 0)]
        if len(reads) == 0:
            continue

        intervals = reads["time_since_last_write_ms"]
        fig, ax = plt.subplots(figsize=(10, 5))
        ax.hist(intervals, bins=50, edgecolor="black", alpha=0.7, color="mediumpurple")
        ax.set_xlabel("Time Since Last Write to Same Key (ms)")
        ax.set_ylabel("Count")
        ax.set_title(f"Read-Write Interval Distribution: {name}\n"
                     f"(n={len(intervals)}, median={intervals.median():.1f}ms)")
        ax.axvline(intervals.median(), color="red", linestyle="--",
                   label=f"median={intervals.median():.1f}ms")
        ax.legend()
        plt.tight_layout()
        out = os.path.join(results_dir, f"{name}-rw-interval.png")
        plt.savefig(out, dpi=150)
        plt.close()
        print(f"Saved {out}")


def plot_stale_read_summary(datasets, results_dir):
    """
    Bar chart showing stale read counts and rates across configurations.
    """
    names, stale_counts, stale_rates, total_reads = [], [], [], []

    for name, df in datasets.items():
        reads = df[df["operation"] == "read"]
        if len(reads) == 0:
            continue
        stale = reads["stale_read"].sum()
        names.append(name)
        stale_counts.append(stale)
        total_reads.append(len(reads))
        stale_rates.append(stale / len(reads) * 100 if len(reads) > 0 else 0)

    if not names:
        return

    fig, (ax1, ax2) = plt.subplots(1, 2, figsize=(14, 6))
    fig.suptitle("Stale Read Analysis", fontsize=14)

    x = np.arange(len(names))
    ax1.bar(x, stale_counts, color="tomato", edgecolor="black")
    ax1.set_xticks(x)
    ax1.set_xticklabels(names, rotation=45, ha="right")
    ax1.set_ylabel("Stale Read Count")
    ax1.set_title("Total Stale Reads")

    ax2.bar(x, stale_rates, color="orange", edgecolor="black")
    ax2.set_xticks(x)
    ax2.set_xticklabels(names, rotation=45, ha="right")
    ax2.set_ylabel("Stale Read Rate (%)")
    ax2.set_title("Stale Read Rate")

    plt.tight_layout()
    out = os.path.join(results_dir, "stale-read-summary.png")
    plt.savefig(out, dpi=150)
    plt.close()
    print(f"Saved {out}")


def print_summary_table(datasets):
    """Print a text summary table of all results."""
    print("\n" + "=" * 90)
    print(f"{'Config':<25} {'Total':>7} {'Reads':>7} {'Writes':>7} "
          f"{'Avg Read ms':>12} {'Avg Write ms':>13} {'Stale':>6} {'Rate':>7}")
    print("-" * 90)

    for name, df in datasets.items():
        reads = df[df["operation"] == "read"]
        writes = df[df["operation"] == "write"]
        stale = reads["stale_read"].sum() if len(reads) > 0 else 0
        rate = stale / len(reads) * 100 if len(reads) > 0 else 0
        avg_r = reads["latency_ms"].mean() if len(reads) > 0 else 0
        avg_w = writes["latency_ms"].mean() if len(writes) > 0 else 0

        print(f"{name:<25} {len(df):>7} {len(reads):>7} {len(writes):>7} "
              f"{avg_r:>12.1f} {avg_w:>13.1f} {stale:>6} {rate:>6.2f}%")

    print("=" * 90)


def main():
    if len(sys.argv) < 2:
        print("Usage: python plot.py <results_directory>")
        print("Example: python plot.py ./results/")
        sys.exit(1)

    results_dir = sys.argv[1]
    if not os.path.isdir(results_dir):
        print(f"Error: {results_dir} is not a directory")
        sys.exit(1)

    datasets = load_results(results_dir)
    print_summary_table(datasets)
    plot_latency_distribution(datasets, results_dir)
    plot_latency_comparison(datasets, results_dir)
    plot_rw_time_interval(datasets, results_dir)
    plot_stale_read_summary(datasets, results_dir)
    print("\nDone! Check the PNG files in", results_dir)


if __name__ == "__main__":
    main()
