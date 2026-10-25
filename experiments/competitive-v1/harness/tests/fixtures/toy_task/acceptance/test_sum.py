import unittest

import testkit


class Sum(unittest.TestCase):
    def test_sums_arguments(self):
        r = testkit.run(["2", "3", "4"])
        self.assertEqual((r.returncode, r.stdout), (0, "9\n"))

    def test_no_arguments(self):
        r = testkit.run([])
        self.assertEqual((r.returncode, r.stdout), (0, "0\n"))
