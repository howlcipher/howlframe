def add(a: int, b: int) -> int:
    return a + b
import unittest
class TestAdd(unittest.TestCase):
    def test_add(self):
        self.assertEqual(add(2, 3), 5)
if __name__ == '__main__':
    unittest.main()
