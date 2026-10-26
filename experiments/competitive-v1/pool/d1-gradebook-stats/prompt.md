# Academic Gradebook and Curved Distribution Analyzer

Build an academic grading CLI tool that calculates weighted course scores from raw assignment submissions, applies structured late penalties and class curves, assigns letter grades, and generates student transcripts along with summary statistics.

## CLI Interface

The executable is located at `./run`. It must implement the `grade` subcommand:

```bash
./run grade --students <students_json> --grades <grades_csv> --rubric <rubric_json> --out <report_json>
```

Arguments:
- `grade`: Required subcommand. If absent or different, exit with code `1`.
- `--students <students_json>`: Path to JSON roster of enrolled students. Required.
- `--grades <grades_csv>`: Path to CSV records of submitted assignments. Required.
- `--rubric <rubric_json>`: Path to grading configuration and rubric parameters. Required.
- `--out <report_json>`: Path where the output transcripts and statistics will be saved. Required.

Options may be passed in any order.

### Exit Codes
- `0`: Execution succeeded and the grade report was written.
- `1`: Invalid CLI arguments, unrecognized subcommand, or missing required options.
- `2`: Any of the specified input files do not exist or cannot be read.

## Input File Formats

### 1. Rubric Configuration (`--rubric`)
A JSON object specifying assignment weights, late policies, curve points, and grade scale thresholds:
```json
{
  "assignment_weights": {
    "hw1": 0.2,
    "hw2": 0.2,
    "midterm": 0.3,
    "final": 0.3
  },
  "late_penalty_percent_per_day": 10.0,
  "max_late_days": 3,
  "curve_points": 2.5,
  "grade_scale": {
    "A": 90.0,
    "B": 80.0,
    "C": 70.0,
    "D": 60.0,
    "F": 0.0
  }
}
```

### 2. Students Roster (`--students`)
A JSON array of student objects:
```json
[
  {"student_id": "S101", "name": "Alice Smith"},
  {"student_id": "S102", "name": "Bob Jones"}
]
```

### 3. Assignment Submissions (`--grades`)
A CSV file with header:
`student_id,assignment_id,score,max_score,days_late`

Example:
```csv
student_id,assignment_id,score,max_score,days_late
S101,hw1,95.0,100.0,0
S101,hw2,80.0,100.0,1
S101,midterm,88.0,100.0,0
S101,final,92.0,100.0,0
```

## Calculation and Grading Logic

1. **Submission Scoring & Late Penalty**:
   - For each submission in the CSV:
     - If `days_late > max_late_days`: `effective_score = 0.0`.
     - Else:
       - `penalty_percent = days_late * late_penalty_percent_per_day`
       - `deduction = max_score * (penalty_percent / 100.0)`
       - `effective_score = max(0.0, score - deduction)`
     - `assignment_percentage = (effective_score / max_score) * 100.0`.
   - If an assignment defined in `assignment_weights` has no record for a student in `--grades`, the student receives `0.0%` for that assignment.
   - If multiple submissions exist for the same student and assignment, use the last one in the CSV.

2. **Final Percentage & Curve**:
   - Compute weighted base score:
     `base_percentage = sum(assignment_percentage * weight for assignment, weight in assignment_weights.items())`
   - Apply curve points:
     `curved_percentage = min(100.0, round(base_percentage + curve_points, 2))`

3. **Letter Grade Assignment**:
   - Evaluate against `grade_scale` thresholds: find the letter grade with the highest threshold where `curved_percentage >= threshold`.
   - Thresholds are tested from highest to lowest score cutoff.

4. **Class Summary Statistics**:
   - `total_students`: count of enrolled students in `--students`.
   - `class_average`: arithmetic mean of all students' `curved_percentage` values, rounded to 2 decimal places (or `0.0` if 0 students).
   - `highest_score`: maximum `curved_percentage` (or `0.0` if 0 students).
   - `lowest_score`: minimum `curved_percentage` (or `0.0` if 0 students).
   - `grade_distribution`: count of each letter grade defined in `grade_scale` across all enrolled students.

## Output Format (`--out`)

Write a JSON object to `<report_json>`:

```json
{
  "transcripts": [
    {
      "student_id": "S101",
      "name": "Alice Smith",
      "final_percentage": 93.3,
      "letter_grade": "A"
    }
  ],
  "class_statistics": {
    "total_students": 1,
    "class_average": 93.3,
    "highest_score": 93.3,
    "lowest_score": 93.3,
    "grade_distribution": {
      "A": 1,
      "B": 0,
      "C": 0,
      "D": 0,
      "F": 0
    }
  }
}
```

Formatting rules:
- `transcripts` must be sorted by `student_id` ascending (code point order).
- `grade_distribution` must contain keys for all letter grades present in `grade_scale`.
