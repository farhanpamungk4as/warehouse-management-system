#!/usr/bin/env python3
"""Populate the warehouse through its public API; Python standard library only.

Run one instance at a time. Existing demo items/balances are preserved. Writes
are never retried automatically; rerunning reads committed state before acting.
"""
import argparse
import json
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode
from urllib.request import Request, urlopen

PRODUCTS = [
    ("Kardus pengiriman ukuran M", "Packaging", "pcs", 120),
    ("Bubble wrap 50 cm", "Packaging", "roll", 80),
    ("Lakban bening 48 mm", "Packaging", "roll", 60),
    ("Label pengiriman thermal", "Stationery", "roll", 40),
    ("Scanner barcode USB", "Electronics", "pcs", 35),
    ("Sarung tangan kerja", "Safety", "pair", 200),
    ("Masker pelindung", "Safety", "box", 150),
    ("Kotak penyimpanan 20 L", "Storage", "pcs", 75),
    ("Stretch film wrapping", "Packaging", "roll", 90),
    ("Timbangan digital 30 kg", "Electronics", "pcs", 50),
]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:18080/api/v1")
    args = parser.parse_args()
    base = args.base_url.rstrip("/")

    def api(method, path, body=None):
        headers = {"Accept": "application/json"}
        data = None
        if body is not None:
            headers["Content-Type"] = "application/json"
            data = json.dumps(body).encode()
        request = Request(base + path, data=data, headers=headers, method=method)
        try:
            with urlopen(request, timeout=15) as response:
                result = json.load(response)
        except HTTPError as error:
            raise RuntimeError(f"{method} {path}: HTTP {error.code}: {error.read().decode()}") from error
        if result.get("success") is not True:
            raise RuntimeError(f"{method} {path}: {result}")
        return result["data"]

    locations = api("GET", "/locations")
    # Create demo racks only when the actual location endpoint is empty.
    if not locations:
        locations = [api("POST", "/locations", {"code": f"DEMO-RACK-{zone}1", "zone": zone, "type": "rack"}) for zone in ("A", "B")]
    print("Locations:", ", ".join(f"{v['code']} (ID {v['id']})" for v in locations))
    created = received = 0
    for index, (name, category, unit, quantity) in enumerate(PRODUCTS, 1):
        sku = f"DEMO-WMS-{index:03d}"
        matches = api("GET", "/items?" + urlencode({"q": sku, "limit": 100}))
        item = next((v for v in matches if v["sku"] == sku), None)
        if item is None:
            item = api("POST", "/items", {"sku": sku, "name": name, "category": category, "unit": unit})
            created += 1
        elif any(item[k] != v for k, v in {"name": name, "category": category, "unit": unit}.items()):
            raise RuntimeError(f"{sku} already belongs to different item data; stopping without modifying it")
        balances = api("GET", f"/stock/{item['id']}")["stocks"]
        if balances:
            print(f"SKIP {sku}: existing stock records preserved")
            continue
        location = locations[(index - 1) % len(locations)]
        api("POST", "/stock/receive", {"item_id": item["id"], "location_id": location["id"], "qty": quantity})
        received += quantity
        print(f"OK   {sku}: {name}, +{quantity} {unit} at {location['code']}")
    print(f"Done: {created} new items, {received} units received.")
    print("Current dashboard (including existing data):")
    print(json.dumps(api("GET", "/dashboard/summary"), indent=2))


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, URLError, ValueError, KeyError) as error:
        print(f"Stopped: {error}", file=sys.stderr)
        print("Earlier successful writes remain saved. Check stock before rerunning; no request was retried automatically.", file=sys.stderr)
        sys.exit(1)
