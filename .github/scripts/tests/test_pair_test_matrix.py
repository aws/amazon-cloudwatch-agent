import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).parents[1] / "pair-test-matrix.py"
SPEC = importlib.util.spec_from_file_location("pair_test_matrix", SCRIPT)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


class PairTestMatrixTest(unittest.TestCase):
    def test_preserves_every_case_exactly_once(self):
        rows = [
            {"testName": "a-1", "test_dir": "./test/a"},
            {"testName": "b-1", "test_dir": "./test/b"},
            {"testName": "a-2", "test_dir": "./test/a"},
            {"testName": "c-1", "test_dir": "./test/c"},
            {"testName": "a-3", "test_dir": "./test/a"},
        ]

        pairs = MODULE.pair_matrix(rows)

        self.assertEqual(3, len(pairs))
        flattened = [case for pair in pairs for case in pair["cases"]]
        self.assertCountEqual(rows, flattened)
        self.assertTrue(all(1 <= len(pair["cases"]) <= 2 for pair in pairs))
        self.assertEqual(["a-1", "a-2"], [case["testName"] for case in pairs[0]["cases"]])

    def test_command_line_writes_compact_json(self):
        rows = [{"testName": f"case-{index}", "test_dir": "./test/a"} for index in range(4)]
        with tempfile.TemporaryDirectory() as temporary_directory:
            input_path = Path(temporary_directory) / "input.json"
            output_path = Path(temporary_directory) / "output.json"
            input_path.write_text(json.dumps(rows), encoding="utf-8")

            import subprocess

            subprocess.run(
                [str(SCRIPT), str(input_path), str(output_path)],
                check=True,
                text=True,
            )

            output = json.loads(output_path.read_text(encoding="utf-8"))
            self.assertEqual(2, len(output))
            self.assertEqual(4, sum(len(pair["cases"]) for pair in output))


if __name__ == "__main__":
    unittest.main()
