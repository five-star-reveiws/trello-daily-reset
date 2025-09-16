package main

import (
	"strings"
	"testing"
	"time"
)

func TestFormatCanvasMetadata(t *testing.T) {
	assignment := CanvasAssignment{
		ID:      12345,
		Name:    "Biology Test 1",
		DueAt:   "2025-09-20T18:00:00Z",
		HTMLURL: "https://alpine.instructure.com/courses/123/assignments/12345",
	}

	tests := []struct {
		name         string
		courseName   string
		submission   *CanvasSubmission
		expectedGrade string
	}{
		{
			name:       "no submission",
			courseName: "Biology",
			submission: nil,
			expectedGrade: "Not graded",
		},
		{
			name:       "good grade",
			courseName: "Biology",
			submission: &CanvasSubmission{Score: floatPtr(95.0)},
			expectedGrade: "95.0%",
		},
		{
			name:       "redo needed",
			courseName: "Biology",
			submission: &CanvasSubmission{Score: floatPtr(85.0)},
			expectedGrade: "85.0% (REDO NEEDED)",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := formatCanvasMetadata(assignment, test.courseName, test.submission)

			if !containsString(result, "Canvas Assignment ID: 12345") {
				t.Errorf("Expected Canvas Assignment ID in metadata")
			}
			if !containsString(result, "Course: "+test.courseName) {
				t.Errorf("Expected course name in metadata")
			}
			if !containsString(result, "Grade: "+test.expectedGrade) {
				t.Errorf("Expected grade '%s' in metadata, got: %s", test.expectedGrade, result)
			}
		})
	}
}

func TestStripCanvasMetadata(t *testing.T) {
	tests := []struct {
		name        string
		description string
		expected    string
	}{
		{
			name:        "no metadata",
			description: "This is a regular description",
			expected:    "This is a regular description",
		},
		{
			name:        "with metadata",
			description: "Assignment description\n\n---\nCanvas Assignment ID: 123\nGrade: 90%",
			expected:    "Assignment description",
		},
		{
			name:        "empty description with metadata",
			description: "\n\n---\nCanvas Assignment ID: 123",
			expected:    "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := stripCanvasMetadata(test.description)
			if result != test.expected {
				t.Errorf("stripCanvasMetadata(%q) = %q, want %q", test.description, result, test.expected)
			}
		})
	}
}

func TestAssignmentFiltering(t *testing.T) {
	now := time.Now()

	assignments := []CanvasAssignment{
		{
			ID:    1,
			Name:  "Past Assignment",
			DueAt: now.AddDate(0, 0, -3).Format(time.RFC3339),
		},
		{
			ID:    2,
			Name:  "Current Assignment",
			DueAt: now.AddDate(0, 0, 5).Format(time.RFC3339),
		},
		{
			ID:    3,
			Name:  "Future Assignment",
			DueAt: now.AddDate(0, 0, 20).Format(time.RFC3339),
		},
		{
			ID:    4,
			Name:  "No Due Date",
			DueAt: "",
		},
	}

	twoWeeksFromNow := now.AddDate(0, 0, 14)
	var filtered []CanvasAssignment

	for _, assignment := range assignments {
		if assignment.DueAt == "" {
			continue
		}

		dueDate, err := time.Parse(time.RFC3339, assignment.DueAt)
		if err != nil {
			continue
		}

		if dueDate.Before(twoWeeksFromNow) && dueDate.After(now.AddDate(0, 0, -1)) {
			filtered = append(filtered, assignment)
		}
	}

	if len(filtered) != 1 {
		t.Errorf("Expected 1 assignment in 2-week window, got %d", len(filtered))
	}

	if filtered[0].Name != "Current Assignment" {
		t.Errorf("Expected 'Current Assignment', got %s", filtered[0].Name)
	}
}

// Helper functions
func floatPtr(f float64) *float64 {
	return &f
}

func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}