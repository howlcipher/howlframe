import unittest

import testkit


class SumHeldOut(unittest.TestCase):
    def test_negative(self):
        r = testkit.run(["-5", "2"])
        self.assertEqual((r.returncode, r.stdout), (0, "-3\n"))

    def test_rejects_non_integer(self):
        r = testkit.run(["1", "x"])
        self.assertEqual(r.returncode, 2)
        self.assertEqual(r.stdout, "")
