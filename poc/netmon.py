#!/usr/bin/env python

import subprocess
import time

def get_udp_received():
    """Run netstat command and extract the 'datagrams received' count."""
    try:
        result = subprocess.run(
            ["netstat", "-s", "-p", "udp"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True
        )
        # Filter for the line containing "datagrams received"
        for line in result.stdout.splitlines():
            if "datagrams received" in line:
                # Extract and return the numeric value
                return int(line.strip().split()[0])
    except Exception as e:
        print(f"Error fetching UDP stats: {e}")
    return None

def main():
    print("Monitoring UDP datagrams received... (Ctrl+C to stop)")
    prev_count = get_udp_received()

    if prev_count is None:
        print("Could not fetch initial UDP datagrams count. Exiting.")
        return

    while True:
        try:
            time.sleep(5)  # Wait for 5 seconds
            current_count = get_udp_received()
            
            if current_count is not None:
                diff = current_count - prev_count
                print(f"UDP datagrams received since last check: {diff}")
                prev_count = current_count
            else:
                print("Could not fetch current UDP datagrams count. Skipping this interval.")

        except KeyboardInterrupt:
            print("\nMonitoring stopped.")
            break

if __name__ == "__main__":
    main()

