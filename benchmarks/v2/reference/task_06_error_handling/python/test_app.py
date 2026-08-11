def divide(a: int, b: int) -> float:
    if b == 0: raise ValueError("Division by zero")
    return a / b
import unittest
class TestDiv(unittest.TestCase):
    def test_div(self):
        with self.assertRaises(ValueError) as cm: divide(10, 0)
        self.assertEqual(str(cm.exception), "Division by zero")
if __name__ == '__main__':
    unittest.main()
