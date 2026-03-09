package testingx

import "testing"

func TestClassifierDetectsAssertionFailure(t *testing.T) {
	classifier := NewClassifier()
	failures := classifier.Classify("--- FAIL: TestAuth\nexpected 200 got 500", "")
	if len(failures) != 1 {
		t.Fatalf("expected one failure classification")
	}
	if failures[0].Code != "test_assertion_failure" {
		t.Fatalf("unexpected failure code: %s", failures[0].Code)
	}
}

func TestClassifierDetectsTimeout(t *testing.T) {
	classifier := NewClassifier()
	failures := classifier.Classify("", "command timed out after 30s")
	if len(failures) != 1 {
		t.Fatalf("expected one failure classification")
	}
	if failures[0].Code != "test_timeout" {
		t.Fatalf("unexpected failure code: %s", failures[0].Code)
	}
}

func TestClassifierDetectsMissingTests(t *testing.T) {
	classifier := NewClassifier()
	failures := classifier.Classify("?   package/foo [no test files]", "")
	if len(failures) != 1 {
		t.Fatalf("expected one failure classification")
	}
	if failures[0].Code != "missing_required_tests" {
		t.Fatalf("unexpected failure code: %s", failures[0].Code)
	}
}
