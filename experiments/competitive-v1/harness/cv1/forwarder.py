"""Loopback TCP to Unix-socket forwarder, run inside a trial jail.

The jail has its own network namespace with only loopback, so an agent CLI inside it cannot reach the host's
metering proxy over TCP. The proxy listens on a Unix socket that the jail bind-mounts; this forwarder exposes it
on 127.0.0.1 inside the jail, where the CLI's base URL points. It copies bytes and nothing else: every policy
decision is made by the proxy, outside the agent's reach.
"""

import argparse
import os
import socket
import sys
import threading


def _pump(src, dst):
    try:
        while True:
            data = src.recv(65536)
            if not data:
                break
            dst.sendall(data)
    except OSError:
        pass
    finally:
        try:
            dst.shutdown(socket.SHUT_WR)
        except OSError:
            pass


def _handle(client, unix_path):
    upstream = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    try:
        upstream.connect(unix_path)
    except OSError:
        client.close()
        upstream.close()
        return
    a = threading.Thread(target=_pump, args=(client, upstream), daemon=True)
    b = threading.Thread(target=_pump, args=(upstream, client), daemon=True)
    a.start()
    b.start()
    a.join()
    b.join()
    client.close()
    upstream.close()


def serve(port, unix_path, ready_file=None):
    listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    listener.bind(("127.0.0.1", port))
    listener.listen(64)
    if ready_file:
        with open(ready_file, "w") as f:
            f.write(str(listener.getsockname()[1]))
    while True:
        client, _ = listener.accept()
        threading.Thread(target=_handle, args=(client, unix_path), daemon=True).start()


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--port", type=int, required=True)
    p.add_argument("--unix", required=True)
    p.add_argument("--ready-file")
    args = p.parse_args(argv)
    if not os.path.exists(args.unix):
        print(f"forwarder: no socket at {args.unix}", file=sys.stderr)
        return 2
    serve(args.port, args.unix, args.ready_file)
    return 0


if __name__ == "__main__":
    sys.exit(main())
