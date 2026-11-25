#!/usr/bin/env python3
"""Собирает статистику репозитория в CSV и пишет сводку STATS.md.

GitHub хранит трафик (просмотры, клоны, источники) только 14 дней, поэтому
этот скрипт раз в неделю дописывает его в файлы, где он копится бессрочно.

Только стандартная библиотека. Использование:
    GITHUB_TOKEN=... python3 scripts/stats.py OWNER/REPO OUT_DIR

Трафик требует токена с правом Administration: read на репозиторий. Если
токен его не даёт (как GITHUB_TOKEN в Actions), трафик пропускается, а
звёзды, форки и скачивания релизов всё равно собираются.
"""

import csv
import datetime as dt
import json
import os
import sys
import urllib.error
import urllib.request

API = "https://api.github.com"


def get(path, token):
    req = urllib.request.Request(API + path)
    req.add_header("Accept", "application/vnd.github+json")
    req.add_header("X-GitHub-Api-Version", "2022-11-28")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    with urllib.request.urlopen(req, timeout=30) as resp:
        return json.load(resp)


def read_rows(path, key):
    """Читает CSV в словарь по ключу (кортеж значений колонок key)."""
    if not os.path.exists(path):
        return {}
    with open(path, newline="") as f:
        return {tuple(r[k] for k in key): r for r in csv.DictReader(f)}


def write_rows(path, fields, rows):
    with open(path, "w", newline="") as f:
        w = csv.DictWriter(f, fieldnames=fields)
        w.writeheader()
        for k in sorted(rows):
            w.writerow(rows[k])


def upsert(path, fields, key, new_rows):
    """Добавляет строки, заменяя существующие с тем же ключом."""
    rows = read_rows(path, key)
    for r in new_rows:
        rows[tuple(str(r[k]) for k in key)] = {f: r.get(f, "") for f in fields}
    write_rows(path, fields, rows)
    return rows


def traffic(repo, token, out):
    """Дневной трафик за последние 14 дней; False, если токен не даёт доступа."""
    try:
        views = get(f"/repos/{repo}/traffic/views", token)["views"]
        clones = get(f"/repos/{repo}/traffic/clones", token)["clones"]
        refs = get(f"/repos/{repo}/traffic/popular/referrers", token)
        paths = get(f"/repos/{repo}/traffic/popular/paths", token)
    except urllib.error.HTTPError as e:
        if e.code in (401, 403, 404):
            print(f"traffic skipped: HTTP {e.code} (token lacks Administration: read)")
            return False
        raise

    days = {}
    for v in views:
        d = days.setdefault(v["timestamp"][:10], {"date": v["timestamp"][:10]})
        d["views"], d["unique_views"] = v["count"], v["uniques"]
    for c in clones:
        d = days.setdefault(c["timestamp"][:10], {"date": c["timestamp"][:10]})
        d["clones"], d["unique_clones"] = c["count"], c["uniques"]
    for d in days.values():
        for k in ("views", "unique_views", "clones", "unique_clones"):
            d.setdefault(k, 0)
    upsert(os.path.join(out, "traffic.csv"),
           ["date", "views", "unique_views", "clones", "unique_clones"],
           ["date"], days.values())

    today = dt.date.today().isoformat()
    upsert(os.path.join(out, "referrers.csv"),
           ["week_of", "referrer", "count", "uniques"], ["week_of", "referrer"],
           [{"week_of": today, **r} for r in refs])
    upsert(os.path.join(out, "paths.csv"),
           ["week_of", "path", "count", "uniques"], ["week_of", "path"],
           [{"week_of": today, "path": p["path"], "count": p["count"],
             "uniques": p["uniques"]} for p in paths])
    return True


def snapshot(repo, token, out):
    today = dt.date.today().isoformat()
    r = get(f"/repos/{repo}", token)
    releases = get(f"/repos/{repo}/releases?per_page=100", token)
    assets = [
        {"date": today, "tag": rel["tag_name"], "asset": a["name"],
         "downloads": a["download_count"]}
        for rel in releases for a in rel["assets"]
    ]
    upsert(os.path.join(out, "downloads.csv"),
           ["date", "tag", "asset", "downloads"], ["date", "tag", "asset"], assets)
    upsert(os.path.join(out, "snapshots.csv"),
           ["date", "stars", "forks", "watchers", "open_issues", "downloads_total"],
           ["date"],
           [{"date": today, "stars": r["stargazers_count"], "forks": r["forks_count"],
             "watchers": r["subscribers_count"], "open_issues": r["open_issues_count"],
             "downloads_total": sum(a["downloads"] for a in assets)}])


def summary(repo, out, has_traffic):
    snaps = read_rows(os.path.join(out, "snapshots.csv"), ["date"])
    last = snaps[max(snaps)] if snaps else {}
    traffic_rows = read_rows(os.path.join(out, "traffic.csv"), ["date"])
    tv = sum(int(r["views"]) for r in traffic_rows.values())
    tc = sum(int(r["clones"]) for r in traffic_rows.values())
    lines = [
        f"# {repo} statistics",
        "",
        f"Updated {dt.date.today().isoformat()} by `.github/workflows/stats.yml`.",
        "",
        "| Metric | Value |",
        "|---|---|",
        f"| Stars | {last.get('stars', 0)} |",
        f"| Forks | {last.get('forks', 0)} |",
        f"| Watchers | {last.get('watchers', 0)} |",
        f"| Release downloads (all time) | {last.get('downloads_total', 0)} |",
        f"| Views since tracking started | {tv} |",
        f"| Clones since tracking started | {tc} |",
        "",
    ]
    if not has_traffic:
        lines += ["Traffic was not collected on this run: the token lacks "
                  "`Administration: read`. Add the `STATS_TOKEN` secret.", ""]
    lines += ["Raw data: `snapshots.csv`, `downloads.csv`, `traffic.csv`, "
              "`referrers.csv`, `paths.csv`.", ""]
    with open(os.path.join(out, "STATS.md"), "w") as f:
        f.write("\n".join(lines))


def main():
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    repo, out = sys.argv[1], sys.argv[2]
    token = os.environ.get("GITHUB_TOKEN", "")
    os.makedirs(out, exist_ok=True)
    snapshot(repo, token, out)
    has_traffic = traffic(repo, token, out)
    summary(repo, out, has_traffic)
    print(open(os.path.join(out, "STATS.md")).read())


if __name__ == "__main__":
    main()
