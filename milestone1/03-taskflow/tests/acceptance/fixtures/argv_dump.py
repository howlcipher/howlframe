import json
import os
import sys

# Prints the exact bytes of every argument (hex), as one JSON list on one line.
print(json.dumps([os.fsencode(a).hex() for a in sys.argv[1:]]))
