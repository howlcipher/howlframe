import sys

try:
    print(sum(int(a) for a in sys.argv[1:]))
except ValueError:
    print("not an integer", file=sys.stderr)
    sys.exit(2)
