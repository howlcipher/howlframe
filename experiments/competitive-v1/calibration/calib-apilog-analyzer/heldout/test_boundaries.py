"""Held-out contract checks focused on boundaries and untested CLI paths."""

import json
import os
import tempfile
import unittest

import testkit


def record(**changes):
    value = {
        "timestamp": 0,
        "service": "svc",
        "client_id": "client",
        "method": "GET",
        "path": "/",
        "status": 200,
        "latency_ms": 0,
        "bytes_sent": 0,
    }
    value.update(changes)
    return value


def invoke(records, args=()):
    return testkit.run(args=args, stdin="\n".join(records) + "\n")


def encoded(*records):
    return [json.dumps(item) for item in records]


class TestBoundaries(unittest.TestCase):
    def test_minimum_valid_field_boundaries(self):
        proc = invoke(encoded(record(timestamp=0, service=" ", client_id=" ",
                                     path="/", status=100)))
        self.assertEqual(proc.returncode, 0, proc.stderr)
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["earliest_timestamp"], 0)
        self.assertEqual(data["summary"]["latest_timestamp"], 0)
        service = data["services"][0]
        self.assertEqual(service["total_requests"], 1)
        self.assertEqual(service["successful_requests"], 0)
        self.assertEqual(service["error_rate"], 0.0)

    def test_status_class_boundaries(self):
        proc = invoke(encoded(
            record(status=199), record(status=200), record(status=399),
            record(status=400), record(status=499), record(status=500),
            record(status=599),
        ))
        self.assertEqual(proc.returncode, 0, proc.stderr)
        service = json.loads(proc.stdout)["services"][0]
        self.assertEqual(service["total_requests"], 7)
        self.assertEqual(service["successful_requests"], 2)
        self.assertEqual(service["client_error_requests"], 2)
        self.assertEqual(service["server_error_requests"], 2)
        self.assertAlmostEqual(service["error_rate"], 4 / 7, places=4)

    def test_zero_latency_and_bytes_remain_zero_in_all_aggregates(self):
        proc = invoke(encoded(record()))
        self.assertEqual(proc.returncode, 0, proc.stderr)
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["total_bytes_sent"], 0)
        self.assertEqual(data["services"][0]["min_latency_ms"], 0)
        self.assertEqual(data["services"][0]["max_latency_ms"], 0)
        self.assertEqual(data["services"][0]["avg_latency_ms"], 0.0)
        self.assertEqual(data["top_clients"][0]["total_bytes"], 0)
        self.assertEqual(data["windows"][0]["avg_latency_ms"], 0.0)

    def test_unknown_record_fields_are_ignored(self):
        proc = invoke(encoded(record(unused={"nested": [1, 2]}, trace_id="x")))
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(json.loads(proc.stdout)["summary"]["valid_records"], 1)

    def test_json_non_objects_are_invalid_and_processing_continues(self):
        lines = ["null", "true", "17", '"text"', "[]", "{}",
                 json.dumps(record(status=201))]
        proc = invoke(lines)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["total_records"], 7)
        self.assertEqual(data["summary"]["invalid_records"], 6)
        self.assertEqual(data["summary"]["valid_records"], 1)

    def test_each_required_field_rejects_missing_value(self):
        required = ("timestamp", "service", "client_id", "method", "path",
                    "status", "latency_ms", "bytes_sent")
        lines = []
        for field in required:
            item = record()
            del item[field]
            lines.append(json.dumps(item))
        proc = invoke(lines)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(json.loads(proc.stdout)["summary"]["invalid_records"], 8)

    def test_numeric_fields_reject_fractional_values_strings_and_booleans(self):
        lines = []
        for field in ("timestamp", "status", "latency_ms", "bytes_sent"):
            for bad in (1.5, "1", True, None):
                lines.append(json.dumps(record(**{field: bad})))
        proc = invoke(lines)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(json.loads(proc.stdout)["summary"]["invalid_records"], 16)

    def test_exponent_notation_with_whole_values_is_accepted(self):
        line = ('{"timestamp":1e0,"service":"svc","client_id":"c",'
                '"method":"GET","path":"/","status":2e2,'
                '"latency_ms":1e1,"bytes_sent":2e1}')
        proc = invoke([line])
        self.assertEqual(proc.returncode, 0, proc.stderr)
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["valid_records"], 1)
        self.assertEqual(data["services"][0]["total_bytes_sent"], 20)
        self.assertEqual(data["services"][0]["avg_latency_ms"], 10.0)

    def test_safe_integer_ceiling_is_inclusive(self):
        maximum = 9007199254740991
        proc = invoke(encoded(record(timestamp=maximum, latency_ms=maximum,
                                     bytes_sent=maximum)))
        self.assertEqual(proc.returncode, 0, proc.stderr)
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["valid_records"], 1)
        self.assertEqual(data["summary"]["earliest_timestamp"], maximum)
        self.assertEqual(data["services"][0]["min_latency_ms"], maximum)
        self.assertEqual(data["summary"]["total_bytes_sent"], maximum)

    def test_values_above_safe_integer_ceiling_are_invalid(self):
        above = 9007199254740992
        lines = [json.dumps(record(**{field: above})) for field in
                 ("timestamp", "latency_ms", "bytes_sent")]
        lines.append(json.dumps(record(timestamp=0, status=2**53)))
        proc = invoke(lines)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(json.loads(proc.stdout)["summary"]["invalid_records"], 4)

    def test_negative_whole_number_values_are_invalid(self):
        lines = [json.dumps(record(**{field: -1.0})) for field in
                 ("timestamp", "latency_ms", "bytes_sent", "status")]
        proc = invoke(lines)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(json.loads(proc.stdout)["summary"]["invalid_records"], 4)

    def test_non_finite_json_numbers_are_invalid(self):
        lines = []
        for field in ("timestamp", "status", "latency_ms", "bytes_sent"):
            for value in ("1e400", "NaN", "Infinity"):
                item = record()
                base = json.dumps(item)
                lines.append(base.replace('"' + field + '": 0',
                                          '"' + field + '": ' + value))
        proc = invoke(lines)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(json.loads(proc.stdout)["summary"]["invalid_records"], 9)

    def test_string_fields_reject_wrong_types_and_empty_values(self):
        lines = []
        for field, bad_values in (
            ("service", ("", 1, None)),
            ("client_id", ("", [], None)),
            ("method", ("get", "TRACE", 1)),
            ("path", ("", "relative", None)),
        ):
            lines.extend(json.dumps(record(**{field: bad})) for bad in bad_values)
        proc = invoke(lines)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(json.loads(proc.stdout)["summary"]["invalid_records"], 12)

    def test_whitespace_only_lines_do_not_break_invalid_record_count(self):
        proc = testkit.run(stdin=" \t\r\n{bad}\n\n" + json.dumps(record()) + "\n")
        self.assertEqual(proc.returncode, 0, proc.stderr)
        summary = json.loads(proc.stdout)["summary"]
        self.assertEqual((summary["total_records"], summary["valid_records"],
                          summary["invalid_records"]), (2, 1, 1))

    def test_sla_threshold_equality_is_not_a_breach(self):
        proc = invoke(encoded(record(status=500, latency_ms=500)),
                      args=["--sla-latency-ms=500", "--error-rate-threshold=1.0"])
        self.assertEqual(proc.returncode, 0, proc.stderr)
        service = json.loads(proc.stdout)["services"][0]
        self.assertFalse(service["sla_breach"])

    def test_error_rate_threshold_equality_is_not_a_breach(self):
        proc = invoke(encoded(record(status=400), record(status=200)),
                      args=["--error-rate-threshold", "0.5"])
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertFalse(json.loads(proc.stdout)["services"][0]["sla_breach"])

    def test_latency_alone_can_trigger_sla_breach_at_zero_threshold(self):
        proc = invoke(encoded(record(latency_ms=1)), args=["--sla-latency-ms=0"])
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertTrue(json.loads(proc.stdout)["services"][0]["sla_breach"])

    def test_window_exact_boundary_starts_next_bucket(self):
        proc = invoke(encoded(record(timestamp=59), record(timestamp=60)),
                      args=["--window-seconds", "60"])
        self.assertEqual(proc.returncode, 0, proc.stderr)
        windows = json.loads(proc.stdout)["windows"]
        self.assertEqual([(w["window_start"], w["window_end"]) for w in windows],
                         [(0, 60), (60, 120)])

    def test_window_aggregation_is_sorted_when_input_is_reversed(self):
        lines = encoded(record(timestamp=601), record(timestamp=1),
                        record(timestamp=301))
        proc = invoke(lines, args=["--window-seconds=300"])
        self.assertEqual(proc.returncode, 0, proc.stderr)
        windows = json.loads(proc.stdout)["windows"]
        self.assertEqual([w["window_start"] for w in windows], [0, 300, 600])

    def test_top_client_ties_use_ascii_lexicographic_order(self):
        proc = invoke(encoded(record(client_id="b"), record(client_id="A"),
                              record(client_id="a")))
        self.assertEqual(proc.returncode, 0, proc.stderr)
        top = json.loads(proc.stdout)["top_clients"]
        self.assertEqual([x["client_id"] for x in top], ["A", "a", "b"])

    def test_top_client_limit_keeps_all_tied_candidates_sorted(self):
        lines = []
        for client_id in reversed([f"c{i:02d}" for i in range(12)]):
            lines.append(json.dumps(record(client_id=client_id)))
        proc = invoke(lines)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        top = json.loads(proc.stdout)["top_clients"]
        self.assertEqual(len(top), 10)
        self.assertEqual([x["client_id"] for x in top],
                         [f"c{i:02d}" for i in range(10)])

    def test_service_order_does_not_depend_on_input_order(self):
        proc = invoke(encoded(record(service="z"), record(service="a"),
                              record(service="m")))
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual([x["service"] for x in json.loads(proc.stdout)["services"]],
                         ["a", "m", "z"])

    def test_all_method_values_are_accepted(self):
        methods = ("GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS")
        proc = invoke(encoded(*(record(method=method) for method in methods)))
        self.assertEqual(proc.returncode, 0, proc.stderr)
        self.assertEqual(json.loads(proc.stdout)["summary"]["valid_records"], 7)

    def test_equals_form_for_file_options(self):
        with tempfile.TemporaryDirectory() as directory:
            source = os.path.join(directory, "input.ndjson")
            destination = os.path.join(directory, "report.json")
            with open(source, "w", encoding="utf-8") as stream:
                stream.write(json.dumps(record()) + "\n")
            proc = testkit.run(args=[f"--input={source}", f"--output={destination}"])
            self.assertEqual(proc.returncode, 0, proc.stderr)
            self.assertEqual(json.load(open(destination, encoding="utf-8"))
                             ["summary"]["valid_records"], 1)

    def test_unreadable_input_path_returns_input_error(self):
        with tempfile.TemporaryDirectory() as directory:
            proc = testkit.run(args=["--input", directory])
        self.assertEqual(proc.returncode, 2)
        self.assertTrue(proc.stderr.strip())

    def test_output_directory_path_returns_usage_error(self):
        with tempfile.TemporaryDirectory() as directory:
            proc = testkit.run(args=["--output", directory], stdin="")
        self.assertEqual(proc.returncode, 1)
        self.assertTrue(proc.stderr.strip())

    def test_cli_rejects_non_integer_and_negative_integer_options(self):
        for args in (["--sla-latency-ms", "1.5"],
                     ["--sla-latency-ms", "-1"],
                     ["--window-seconds", "1.5"],
                     ["--window-seconds", "-1"]):
            with self.subTest(args=args):
                proc = testkit.run(args=args)
                self.assertEqual(proc.returncode, 1)
                self.assertTrue(proc.stderr.strip())

    def test_cli_rejects_out_of_range_or_non_numeric_error_thresholds(self):
        for value in ("-0.01", "1.01", "nan", "inf", "not-a-number"):
            with self.subTest(value=value):
                proc = testkit.run(args=["--error-rate-threshold", value])
                self.assertEqual(proc.returncode, 1)
                self.assertTrue(proc.stderr.strip())

    def test_large_reasonable_input_is_fully_counted(self):
        lines = (json.dumps(record(timestamp=i, bytes_sent=i % 17,
                                   latency_ms=i % 13)) for i in range(1200))
        proc = testkit.run(stdin="\n".join(lines) + "\n", timeout=10)
        self.assertEqual(proc.returncode, 0, proc.stderr)
        data = json.loads(proc.stdout)
        self.assertEqual(data["summary"]["total_records"], 1200)
        self.assertEqual(data["summary"]["valid_records"], 1200)
        self.assertEqual(data["summary"]["total_bytes_sent"], sum(i % 17 for i in range(1200)))


if __name__ == "__main__":
    unittest.main()
