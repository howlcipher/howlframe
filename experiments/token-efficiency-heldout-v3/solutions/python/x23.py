from pathlib import Path

dept = dict(l.split(",") for l in Path("in/dept.csv").read_text().splitlines()[1:])
out = ["id,name,dept"]
for l in Path("in/emp.csv").read_text().splitlines()[1:]:
    out.append(f"{l},{dept.get(l.split(',')[0], '')}")
Path("out/result.csv").write_text("\n".join(out) + "\n")
