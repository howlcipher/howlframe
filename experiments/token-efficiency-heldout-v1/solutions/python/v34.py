from pathlib import Path

edges = dict(line.split(">") for line in Path("in/edges.txt").read_text().split("\n"))
node = next(n for n in edges if n not in edges.values())
chain = [node]
while node in edges:
    node = edges[node]
    chain.append(node)
Path("out/result.txt").write_text(">".join(chain) + "\n")
