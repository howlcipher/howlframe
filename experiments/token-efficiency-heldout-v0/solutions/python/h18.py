from pathlib import Path

emails = Path("in/emails.txt").read_text().split("\n")
domains = [email.split("@")[1] for email in emails]
Path("out/result.txt").write_text("\n".join(domains) + "\n")
