#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(2468)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    rubric = {
        "assignment_weights": {
            "hw1": 0.15,
            "hw2": 0.15,
            "quiz": 0.20,
            "midterm": 0.25,
            "final": 0.25
        },
        "late_penalty_percent_per_day": 5.0,
        "max_late_days": 4,
        "curve_points": 3.0,
        "grade_scale": {
            "A": 90.0,
            "B": 80.0,
            "C": 70.0,
            "D": 60.0,
            "F": 0.0
        }
    }
    with open(os.path.join(dir_path, "rubric.json"), "w") as f:
        json.dump(rubric, f, indent=2)

    students = []
    grade_rows = ["student_id,assignment_id,score,max_score,days_late"]
    assignments = ["hw1", "hw2", "quiz", "midterm", "final"]

    for i in range(1, 151):
        sid = f"STU-{i:04d}"
        students.append({"student_id": sid, "name": f"Student {i}"})
        for assign in assignments:
            # 5% chance student didn't submit
            if random.random() < 0.05:
                continue
            max_s = 100.0 if assign in ("midterm", "final") else 50.0
            raw_s = round(random.uniform(25.0, max_s), 1)
            days_late = random.choices([0, 1, 2, 5], weights=[0.7, 0.15, 0.1, 0.05])[0]
            grade_rows.append(f"{sid},{assign},{raw_s},{max_s},{days_late}")

    with open(os.path.join(dir_path, "students.json"), "w") as f:
        json.dump(students, f, indent=2)

    with open(os.path.join(dir_path, "grades.csv"), "w") as f:
        f.write("\n".join(grade_rows) + "\n")

if __name__ == "__main__":
    main()
