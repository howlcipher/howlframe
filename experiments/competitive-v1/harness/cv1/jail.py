"""Bubblewrap jails for agent runs and for scoring (protocol §4, Revision 3 G4(b)).

Every jail starts from the same pinned read-only root (an Ubuntu base image with no language toolchain), adds
exactly one arm toolchain and its offline documentation, and has its own network namespace with loopback only.
The only route out is the proxy socket, which is bind-mounted for agent runs and absent when scoring.

Layout inside a jail:

    /                  pinned base root, read-only
    /opt/toolchain     the arm's toolchain, read-only (its bin/ is first on PATH)
    /opt/docs          the arm's offline documentation, read-only
    /opt/agent         the agent CLI, read-only (agent runs only)
    /opt/cv1/bin       task-complete, run-acceptance-tests, the forwarder, read-only
    /opt/cv1/testpy    the Python that runs the black-box tests, read-only, not on PATH
    /tests             frozen acceptance tests, read-only
    /workspace         the agent's writable workspace
    /home/agent        a fresh, writable HOME holding only the CLI's generated configuration
    /run/cv1           writable control directory (completion marker)
    /run/cv1-proxy     read-only directory holding only the proxy's Unix socket (agent runs only)
"""

import os

AGENT_UID = 1000
AGENT_GID = 1000

PASSWD = "root:x:0:0:root:/root:/usr/sbin/nologin\nagent:x:{uid}:{gid}:agent:/home/agent:/bin/bash\n"
GROUP = "root:x:0:\nagent:x:{gid}:\nnogroup:x:65534:\n"
HOSTS = "127.0.0.1 localhost\n::1 localhost\n"
ROOT_SKIP = {"dev", "proc", "sys", "tmp", "run", "opt", "home", "root", "mnt", "media", "srv", "boot", "lost+found"}


def write_identity_files(directory):
    """Write passwd, group and hosts files for the jail; returns the directory (JailSpec.identity_dir)."""
    os.makedirs(directory, exist_ok=True)
    for name, text in (("passwd", PASSWD), ("group", GROUP), ("hosts", HOSTS)):
        with open(os.path.join(directory, name), "w") as f:
            f.write(text.format(uid=AGENT_UID, gid=AGENT_GID))
    return directory


class JailSpec:
    def __init__(self, root, identity_dir, workspace, toolchain=None, docs=None, tests=None, agent_dir=None,
                 cv1_bin=None, testpy=None, home=None, control=None, proxy_socket_dir=None, workspace_readonly=False,
                 extra_ro=None):
        self.root = root
        self.identity_dir = identity_dir
        self.workspace = workspace
        self.toolchain = toolchain
        self.docs = docs
        self.tests = tests
        self.agent_dir = agent_dir
        self.cv1_bin = cv1_bin
        self.testpy = testpy
        self.home = home
        self.control = control
        self.proxy_socket_dir = proxy_socket_dir
        self.workspace_readonly = workspace_readonly
        self.extra_ro = extra_ro or []


def jail_path():
    return "/opt/toolchain/bin:/opt/cv1/bin:/usr/local/bin:/usr/bin:/bin"


def bwrap_argv(spec, command, env, cwd="/workspace", bwrap="bwrap"):
    """The bwrap command line. `env` replaces the environment completely (--clearenv)."""
    a = [bwrap, "--unshare-all", "--die-with-parent", "--new-session", "--clearenv",
         "--uid", str(AGENT_UID), "--gid", str(AGENT_GID), "--hostname", "cv1"]
    # The jail's / is bwrap's own tmpfs; each top-level entry of the pinned root is bound read-only into it, so
    # mount points for /workspace and /tests can be created. / itself is remounted read-only at the end.
    for name in sorted(os.listdir(spec.root)):
        if name in ROOT_SKIP:
            continue
        src = os.path.join(spec.root, name)
        if os.path.islink(src):
            a += ["--symlink", os.readlink(src), "/" + name]
        elif os.path.isdir(src):
            a += ["--ro-bind", src, "/" + name]
    a += ["--proc", "/proc", "--dev", "/dev", "--tmpfs", "/tmp", "--tmpfs", "/run", "--tmpfs", "/opt",
          "--tmpfs", "/home"]
    ident = spec.identity_dir
    a += ["--ro-bind", os.path.join(ident, "passwd"), "/etc/passwd",
          "--ro-bind", os.path.join(ident, "group"), "/etc/group",
          "--ro-bind", os.path.join(ident, "hosts"), "/etc/hosts"]
    for src, dst in ((spec.toolchain, "/opt/toolchain"), (spec.docs, "/opt/docs"), (spec.agent_dir, "/opt/agent"),
                     (spec.cv1_bin, "/opt/cv1/bin"), (spec.testpy, "/opt/cv1/testpy"), (spec.tests, "/tests")):
        if src:
            a += ["--ro-bind", src, dst]
    a += ["--ro-bind" if spec.workspace_readonly else "--bind", spec.workspace, "/workspace"]
    if spec.home:
        a += ["--bind", spec.home, "/home/agent"]
    else:
        a += ["--dir", "/home/agent"]
    if spec.control:
        a += ["--bind", spec.control, "/run/cv1"]
    if spec.proxy_socket_dir:
        a += ["--ro-bind", spec.proxy_socket_dir, "/run/cv1-proxy"]
    for src, dst in spec.extra_ro:
        a += ["--ro-bind", src, dst]
    a += ["--remount-ro", "/"]
    for k, v in sorted(env.items()):
        a += ["--setenv", k, v]
    a += ["--chdir", cwd, "--"]
    return a + list(command)


def limits_prefix(memory_max="8G", tasks_max=1024, cpus=None, use_systemd=True):
    """Resource limits wrapped around bwrap: a transient systemd scope (memory, tasks) and CPU pinning.
    The same limits apply to every arm; the values are recorded with each run."""
    prefix = []
    if use_systemd:
        prefix += ["systemd-run", "--user", "--scope", "--quiet", "-p", f"MemoryMax={memory_max}",
                   "-p", "MemorySwapMax=0", "-p", f"TasksMax={tasks_max}", "--"]
    if cpus:
        prefix += ["taskset", "-c", cpus]
    return prefix
