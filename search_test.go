package main

import (
	"testing"
)

func TestNormalizeString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello world"},
		{"  TRIMMED  ", "trimmed"},
		{"CamelCase", "camelcase"},
		{"Special-Characters_123", "special-characters_123"},
		{"", ""},
	}

	for _, test := range tests {
		result := normalizeString(test.input)
		if result != test.expected {
			t.Errorf("normalizeString(%q) = %q, want %q", test.input, result, test.expected)
		}
	}
}

func TestFindBoardByName(t *testing.T) {
	boards := []Board{
		{ID: "1", Name: "Mac's Board", URL: ""},
		{ID: "2", Name: "Family Tasks", URL: ""},
		{ID: "3", Name: "Work Stuff", URL: ""},
		{ID: "4", Name: "After Work", URL: ""},
	}

	tests := []struct {
		name      string
		boardName string
		expected  *Board
		shouldErr bool
	}{
		{
			name:      "exact match",
			boardName: "Mac's Board",
			expected:  &boards[0],
			shouldErr: false,
		},
		{
			name:      "case insensitive",
			boardName: "MAC'S BOARD",
			expected:  &boards[0],
			shouldErr: false,
		},
		{
			name:      "partial match",
			boardName: "Family",
			expected:  &boards[1],
			shouldErr: false,
		},
		{
			name:      "whitespace handling",
			boardName: "  Family Tasks  ",
			expected:  &boards[1],
			shouldErr: false,
		},
		{
			name:      "not found",
			boardName: "Nonexistent",
			expected:  nil,
			shouldErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := findBoardByName(boards, test.boardName)

			if test.shouldErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.ID != test.expected.ID {
				t.Errorf("findBoardByName(%q) = %v, want %v", test.boardName, result, test.expected)
			}
		})
	}
}

func TestFindListByName(t *testing.T) {
	lists := []List{
		{ID: "1", Name: "To Do", BoardID: "board1"},
		{ID: "2", Name: "In Progress", BoardID: "board1"},
		{ID: "3", Name: "Done - To Be Reviewed by Dad", BoardID: "board1"},
		{ID: "4", Name: "Sprint Planning", BoardID: "board2"},
	}

	tests := []struct {
		name      string
		boardID   string
		listName  string
		expected  *List
		shouldErr bool
	}{
		{
			name:      "exact match",
			boardID:   "board1",
			listName:  "To Do",
			expected:  &lists[0],
			shouldErr: false,
		},
		{
			name:      "case insensitive",
			boardID:   "board1",
			listName:  "IN PROGRESS",
			expected:  &lists[1],
			shouldErr: false,
		},
		{
			name:      "partial match with spaces",
			boardID:   "board1",
			listName:  "Done",
			expected:  &lists[2],
			shouldErr: false,
		},
		{
			name:      "wrong board",
			boardID:   "board2",
			listName:  "To Do",
			expected:  nil,
			shouldErr: true,
		},
		{
			name:      "not found",
			boardID:   "board1",
			listName:  "Nonexistent",
			expected:  nil,
			shouldErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := findListByName(lists, test.boardID, test.listName)

			if test.shouldErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.ID != test.expected.ID {
				t.Errorf("findListByName(%q, %q) = %v, want %v", test.boardID, test.listName, result, test.expected)
			}
		})
	}
}