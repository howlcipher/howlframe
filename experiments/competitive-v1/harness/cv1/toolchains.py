"""Fetch, verify and unpack the pinned base root, toolchains and offline documentation into a cache directory.

Nothing is installed system-wide. Each download is checked against `toolchains.json`; where the publisher gives
no digest, the digest seen is written to `<cache>/lock.json` and the freeze pins it. Each unpacked directory gets
a tree digest (sorted relative paths, modes, and file or link-target SHA-256), which is what the freeze records
as the jail hash.

    python3 -m cv1.toolchains --cache ~/.cache/howlframe-cv1 [--howlframe-repo PATH --howlframe-commit SHA]
"""

import argparse
import hashlib
import html.parser
import json
import os
import shutil
import stat
import subprocess
import sys
import tarfile
import urllib.error
import urllib.request

HARNESS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
MANIFEST = os.path.join(HARNESS, "toolchains.json")
USER_AGENT = "howlframe-cv1-harness/1"


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for block in iter(lambda: f.read(1 << 20), b""):
            h.update(block)
    return h.hexdigest()


def tree_digest(root):
    """Digest of a directory tree: relative path, file type, executable bit and content or link target."""
    h = hashlib.sha256()
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames.sort()
        for name in sorted(filenames + [d for d in dirnames if os.path.islink(os.path.join(dirpath, d))]):
            full = os.path.join(dirpath, name)
            rel = os.path.relpath(full, root)
            st = os.lstat(full)
            if stat.S_ISLNK(st.st_mode):
                h.update(f"L {rel} -> {os.readlink(full)}\n".encode())
            elif stat.S_ISREG(st.st_mode):
                h.update(f"F {rel} {'x' if st.st_mode & 0o111 else '-'} {sha256_file(full)}\n".encode())
    return h.hexdigest()


def download(url, dest):
    req = urllib.request.Request(url, headers={"User-Agent": USER_AGENT})
    tmp = dest + ".part"
    with urllib.request.urlopen(req, timeout=120) as r, open(tmp, "wb") as f:
        shutil.copyfileobj(r, f)
    os.replace(tmp, dest)


def fetch_verified(url, expected, downloads):
    os.makedirs(downloads, exist_ok=True)
    name = url.rsplit("/", 1)[-1].replace("%2B", "+")
    dest = os.path.join(downloads, name)
    if not os.path.exists(dest):
        download(url, dest)
    got = sha256_file(dest)
    if expected and got != expected:
        os.unlink(dest)
        raise SystemExit(f"checksum mismatch for {url}: expected {expected}, got {got}")
    return dest, got


def _safe_members(tar, strip):
    for m in tar.getmembers():
        if m.isdev() or m.isfifo():
            continue
        name = m.name.removeprefix("./")
        if strip:
            if name == strip or not name.startswith(strip + "/"):
                continue
            name = name[len(strip) + 1:]
        if not name or name.startswith("/") or ".." in name.split("/"):
            continue
        m.name = name
        if m.islnk():
            link = m.linkname.removeprefix("./")
            if strip and link.startswith(strip + "/"):
                link = link[len(strip) + 1:]
            m.linkname = link
        yield m


def unpack(archive, target, strip=None):
    if os.path.exists(target):
        shutil.rmtree(target)
    os.makedirs(target)
    with tarfile.open(archive) as tar:
        # The "tar" filter keeps absolute symlinks (the base root has many) but refuses unsafe paths; members are
        # also pre-filtered above, and device nodes are dropped because the jail mounts its own /dev.
        tar.extractall(target, members=list(_safe_members(tar, strip)), filter="tar")
    return target


class _LinkParser(html.parser.HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []

    def handle_starttag(self, tag, attrs):
        if tag == "a":
            for k, v in attrs:
                if k == "href" and v:
                    self.links.append(v)


def fetch_node_docs(index_url, suffix, target):
    """Node publishes each API page as .html and .md; the index page links the .html names."""
    req = urllib.request.Request(index_url, headers={"User-Agent": USER_AGENT})
    with urllib.request.urlopen(req, timeout=60) as r:
        parser = _LinkParser()
        parser.feed(r.read().decode("utf-8", "replace"))
    pages = sorted({h[:-len(".html")] for h in parser.links
                    if h.endswith(".html") and "/" not in h and "#" not in h})
    if os.path.exists(target):
        shutil.rmtree(target)
    os.makedirs(target)
    names = []
    for page in pages:
        try:
            download(index_url + page + suffix, os.path.join(target, page + suffix))
            names.append(page + suffix)
        except urllib.error.HTTPError as e:
            if e.code != 404:
                raise
    return names


def build_howlframe(repo, commit, target):
    """Build the HowlFrame compiler from a clean worktree of `commit` into target/bin/howlframe."""
    work = target + ".src"
    if os.path.exists(work):
        subprocess.run(["git", "-C", repo, "worktree", "remove", "--force", work], check=False)
        shutil.rmtree(work, ignore_errors=True)
    subprocess.run(["git", "-C", repo, "worktree", "add", "--detach", work, commit], check=True)
    try:
        os.makedirs(os.path.join(target, "bin"), exist_ok=True)
        env = dict(os.environ, CGO_ENABLED="0", GOFLAGS="-trimpath")
        subprocess.run(["go", "build", "-o", os.path.join(target, "bin", "howlframe"), "howlframe.go"], cwd=work,
                       env=env, check=True)
        go_version = subprocess.run(["go", "version"], capture_output=True, text=True, check=True).stdout.strip()
    finally:
        subprocess.run(["git", "-C", repo, "worktree", "remove", "--force", work], check=False)
    return go_version


def howlframe_docs(repo, commit, skill, skill_source, target):
    if os.path.exists(target):
        shutil.rmtree(target)
    os.makedirs(target)
    for path in ("README.md", "docs/cli.md"):
        data = subprocess.run(["git", "-C", repo, "show", f"{commit}:{path}"], capture_output=True, check=True).stdout
        out = os.path.join(target, path)
        os.makedirs(os.path.dirname(out), exist_ok=True)
        with open(out, "wb") as f:
            f.write(data)
    archive = subprocess.run(["git", "-C", repo, "archive", commit, "docs/reference"], capture_output=True,
                             check=True).stdout
    tmp = os.path.join(target, ".ref.tar")
    with open(tmp, "wb") as f:
        f.write(archive)
    with tarfile.open(tmp) as tar:
        tar.extractall(target, filter="data")
    os.unlink(tmp)
    if skill_source:
        got = sha256_file(skill_source)
        if got != skill["sha256"]:
            raise SystemExit(f"HowlFrame skill text digest {got} does not match the pin {skill['sha256']}")
        shutil.copy(skill_source, os.path.join(target, "SKILL.md"))


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--cache", required=True)
    p.add_argument("--only", nargs="*", help="subset: base python go javascript docs howlframe")
    p.add_argument("--howlframe-repo")
    p.add_argument("--howlframe-commit")
    p.add_argument("--skill-file", help="local copy of the pinned HowlFrame skill text (digest-checked)")
    args = p.parse_args(argv)
    with open(MANIFEST) as f:
        m = json.load(f)
    cache = os.path.abspath(args.cache)
    downloads = os.path.join(cache, "downloads")
    lock_path = os.path.join(cache, "lock.json")
    lock = {}
    if os.path.exists(lock_path):
        with open(lock_path) as f:
            lock = json.load(f)
    want = set(args.only or ["base", "python", "go", "javascript", "docs"])

    if "base" in want:
        b = m["base_root"]
        archive, digest = fetch_verified(b["url"], b["sha256"], downloads)
        root = unpack(archive, os.path.join(cache, "root"))
        lock["base_root"] = {"archive_sha256": digest, "tree_sha256": tree_digest(root)}
    for arm in ("python", "go", "javascript"):
        if arm in want:
            t = m["toolchains"][arm]
            archive, digest = fetch_verified(t["url"], t["sha256"], downloads)
            d = unpack(archive, os.path.join(cache, "toolchains", arm), t["strip"])
            lock[f"toolchain_{arm}"] = {"archive_sha256": digest, "tree_sha256": tree_digest(d),
                                        "version": t["version"]}
    if "docs" in want:
        d = m["docs"]
        archive, digest = fetch_verified(d["python"]["url"], d["python"]["sha256"], downloads)
        pydocs = unpack(archive, os.path.join(cache, "docs", "python"), d["python"]["strip"])
        lock["docs_python"] = {"archive_sha256": digest, "tree_sha256": tree_digest(pydocs)}
        jsdir = os.path.join(cache, "docs", "javascript")
        names = fetch_node_docs(d["javascript"]["index_url"], d["javascript"]["suffix"], jsdir)
        lock["docs_javascript"] = {"files": len(names), "tree_sha256": tree_digest(jsdir)}
        godir = os.path.join(cache, "docs", "go")
        os.makedirs(godir, exist_ok=True)
        with open(os.path.join(godir, "README.txt"), "w") as f:
            f.write("Go documentation is available offline with `go doc <package>` and in the standard-library\n"
                    "sources under /opt/toolchain/src.\n")
        lock["docs_go"] = {"tree_sha256": tree_digest(godir)}
    if "howlframe" in want:
        if not (args.howlframe_repo and args.howlframe_commit):
            raise SystemExit("--howlframe-repo and --howlframe-commit are required for the howlframe arm")
        target = os.path.join(cache, "toolchains", "howlframe")
        go_version = build_howlframe(args.howlframe_repo, args.howlframe_commit, target)
        hdocs = os.path.join(cache, "docs", "howlframe")
        howlframe_docs(args.howlframe_repo, args.howlframe_commit, m["docs"]["howlframe"]["skill"], args.skill_file,
                       hdocs)
        lock["toolchain_howlframe"] = {"commit": args.howlframe_commit, "built_with": go_version,
                                       "tree_sha256": tree_digest(target)}
        lock["docs_howlframe"] = {"commit": args.howlframe_commit, "skill_included": bool(args.skill_file),
                                  "tree_sha256": tree_digest(hdocs)}
    with open(lock_path, "w") as f:
        json.dump(lock, f, indent=2, sort_keys=True)
        f.write("\n")
    print(json.dumps(lock, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
