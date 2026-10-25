import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestGradebookStats(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.students_path = os.path.join(self.temp_dir.name, "students.json")
        self.grades_path = os.path.join(self.temp_dir.name, "grades.csv")
        self.rubric_path = os.path.join(self.temp_dir.name, "rubric.json")
        self.out_path = os.path.join(self.temp_dir.name, "report.json")

        self.rubric = {
            "assignment_weights": {"hw1": 0.5, "exam": 0.5},
            "late_penalty_percent_per_day": 10.0,
            "max_late_days": 2,
            "curve_points": 5.0,
            "grade_scale": {
                "A": 90.0,
                "B": 80.0,
                "C": 70.0,
                "D": 60.0,
                "F": 0.0
            }
        }
        with open(self.rubric_path, "w") as f:
            json.dump(self.rubric, f)

    def tearDown(self):
        self.temp_dir.cleanup()

    def _write_inputs(self, students, grade_rows):
        with open(self.students_path, "w") as f:
            json.dump(students, f)
        with open(self.grades_path, "w") as f:
            f.write("student_id,assignment_id,score,max_score,days_late\n")
            for r in grade_rows:
                f.write(r + "\n")

    def test_basic_grading_no_penalties(self):
        self._write_inputs(
            [{"student_id": "S1", "name": "Alice"}],
            ["S1,hw1,100.0,100.0,0", "S1,exam,100.0,100.0,0"]
        )
        res = testkit.run(["grade", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        t = out["transcripts"][0]
        self.assertEqual(t["student_id"], "S1")
        self.assertEqual(t["final_percentage"], 100.0) # capped at 100 with curve
        self.assertEqual(t["letter_grade"], "A")

    def test_late_penalty_application(self):
        # 1 day late on 100 max_score at 10% penalty = -10 points -> 90 - 10 = 80 -> 80%
        # exam: 100% -> base = 0.5*80 + 0.5*100 = 90. + 5 curve = 95.0
        self._write_inputs(
            [{"student_id": "S1", "name": "Alice"}],
            ["S1,hw1,90.0,100.0,1", "S1,exam,100.0,100.0,0"]
        )
        res = testkit.run(["grade", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["transcripts"][0]["final_percentage"], 95.0)

    def test_late_days_exceeding_max_yields_zero(self):
        # max_late_days is 2; 3 days late yields 0.0 for hw1
        self._write_inputs(
            [{"student_id": "S1", "name": "Alice"}],
            ["S1,hw1,100.0,100.0,3", "S1,exam,100.0,100.0,0"]
        )
        res = testkit.run(["grade", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        # 0.5 * 0 + 0.5 * 100 + 5.0 = 55.0 -> letter grade F
        self.assertEqual(out["transcripts"][0]["final_percentage"], 55.0)
        self.assertEqual(out["transcripts"][0]["letter_grade"], "F")

    def test_missing_assignment_counts_as_zero(self):
        self._write_inputs(
            [{"student_id": "S1", "name": "Alice"}],
            ["S1,exam,100.0,100.0,0"] # missing hw1
        )
        res = testkit.run(["grade", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["transcripts"][0]["final_percentage"], 55.0)

    def test_curve_points_addition(self):
        self._write_inputs(
            [{"student_id": "S1", "name": "Alice"}],
            ["S1,hw1,80.0,100.0,0", "S1,exam,80.0,100.0,0"]
        )
        res = testkit.run(["grade", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        # 80.0 + 5.0 curve = 85.0 -> B
        self.assertEqual(out["transcripts"][0]["final_percentage"], 85.0)
        self.assertEqual(out["transcripts"][0]["letter_grade"], "B")

    def test_curved_percentage_cap_at_100(self):
        self._write_inputs(
            [{"student_id": "S1", "name": "Alice"}],
            ["S1,hw1,98.0,100.0,0", "S1,exam,100.0,100.0,0"]
        )
        # base: 99.0 + 5.0 = 104.0 -> capped at 100.0
        res = testkit.run(["grade", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["transcripts"][0]["final_percentage"], 100.0)

    def test_letter_grade_boundary_conditions(self):
        # exactly 90.0 -> A
        # exactly 89.9 -> B
        self._write_inputs(
            [
                {"student_id": "S1", "name": "Alice"},
                {"student_id": "S2", "name": "Bob"}
            ],
            [
                "S1,hw1,85.0,100.0,0", "S1,exam,85.0,100.0,0", # 85 + 5 = 90.0
                "S2,hw1,84.9,100.0,0", "S2,exam,84.9,100.0,0"  # 84.9 + 5 = 89.9
            ]
        )
        res = testkit.run(["grade", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["transcripts"][0]["letter_grade"], "A")
        self.assertEqual(out["transcripts"][1]["letter_grade"], "B")

    def test_class_statistics_distribution(self):
        self._write_inputs(
            [
                {"student_id": "S1", "name": "Alice"},
                {"student_id": "S2", "name": "Bob"}
            ],
            [
                "S1,hw1,85.0,100.0,0", "S1,exam,85.0,100.0,0", # 90.0
                "S2,hw1,75.0,100.0,0", "S2,exam,75.0,100.0,0"  # 80.0
            ]
        )
        res = testkit.run(["grade", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        stats = out["class_statistics"]
        self.assertEqual(stats["total_students"], 2)
        self.assertEqual(stats["class_average"], 85.0)
        self.assertEqual(stats["highest_score"], 90.0)
        self.assertEqual(stats["lowest_score"], 80.0)
        self.assertEqual(stats["grade_distribution"]["A"], 1)
        self.assertEqual(stats["grade_distribution"]["B"], 1)
        self.assertEqual(stats["grade_distribution"]["C"], 0)

    def test_transcripts_sorting(self):
        self._write_inputs(
            [
                {"student_id": "STU-99", "name": "Zoe"},
                {"student_id": "STU-01", "name": "Alex"}
            ],
            []
        )
        res = testkit.run(["grade", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["transcripts"][0]["student_id"], "STU-01")
        self.assertEqual(out["transcripts"][1]["student_id"], "STU-99")

    def test_multiple_submissions_uses_last(self):
        self._write_inputs(
            [{"student_id": "S1", "name": "Alice"}],
            [
                "S1,hw1,50.0,100.0,0",
                "S1,hw1,100.0,100.0,0", # last one should override
                "S1,exam,100.0,100.0,0"
            ]
        )
        res = testkit.run(["grade", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["transcripts"][0]["final_percentage"], 100.0)

    def test_missing_cli_flags_exit_1(self):
        res = testkit.run(["grade", "--students", self.students_path])
        self.assertEqual(res.returncode, 1)

    def test_wrong_subcommand_exit_1(self):
        res = testkit.run(["calc", "--students", self.students_path, "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 1)

    def test_missing_files_exit_2(self):
        res = testkit.run(["grade", "--students", "/missing/students.json", "--grades", self.grades_path, "--rubric", self.rubric_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 2)


if __name__ == "__main__":
    unittest.main()
