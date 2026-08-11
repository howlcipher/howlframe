from typing import List, Dict
def process_scores(items: List[Dict[str, int]]) -> List[str]:
    filtered = [i for i in items if i['Score'] >= 50]
    filtered.sort(key=lambda x: x['Score'], reverse=True)
    return [i['Name'] for i in filtered]
import unittest
class TestScores(unittest.TestCase):
    def test_process(self):
        self.assertEqual(process_scores([{"Name": "Alice", "Score": 40}, {"Name": "Bob", "Score": 80}, {"Name": "Charlie", "Score": 90}]), ["Charlie", "Bob"])
if __name__ == '__main__':
    unittest.main()
