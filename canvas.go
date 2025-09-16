package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CanvasClient struct {
	APIToken string
	BaseURL  string
}

type CanvasUser struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	LoginID  string `json:"login_id"`
}

type CanvasCourse struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Code string `json:"course_code"`
}

type CanvasAssignment struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DueAt       string `json:"due_at"`
	CourseID    int    `json:"course_id"`
	HTMLURL     string `json:"html_url"`
}

type CanvasSubmission struct {
	Score      *float64 `json:"score"`
	Grade      string   `json:"grade"`
	WorkflowState string `json:"workflow_state"`
}

func NewCanvasClient(apiToken, baseURL string) *CanvasClient {
	return &CanvasClient{
		APIToken: apiToken,
		BaseURL:  baseURL,
	}
}

func (c *CanvasClient) makeRequest(endpoint string) ([]byte, error) {
	u, err := url.Parse(c.BaseURL + "/api/v1" + endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Canvas uses Authorization header with Bearer token
	req.Header.Set("Authorization", "Bearer "+c.APIToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Canvas API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

func (c *CanvasClient) GetCurrentUser() (*CanvasUser, error) {
	body, err := c.makeRequest("/users/self")
	if err != nil {
		return nil, err
	}

	var user CanvasUser
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	return &user, nil
}

func (c *CanvasClient) TestConnection() error {
	user, err := c.GetCurrentUser()
	if err != nil {
		return fmt.Errorf("failed to connect to Canvas: %w", err)
	}

	fmt.Printf("✅ Canvas connection successful!\n")
	fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
	fmt.Printf("Login ID: %s\n", user.LoginID)
	fmt.Printf("Canvas User ID: %d\n", user.ID)

	return nil
}

func (c *CanvasClient) GetCourses() ([]CanvasCourse, error) {
	body, err := c.makeRequest("/courses?enrollment_state=active&per_page=100")
	if err != nil {
		return nil, err
	}

	var courses []CanvasCourse
	if err := json.Unmarshal(body, &courses); err != nil {
		return nil, fmt.Errorf("failed to unmarshal courses: %w", err)
	}

	return courses, nil
}

func (c *CanvasClient) GetAssignments(courseID int) ([]CanvasAssignment, error) {
	endpoint := fmt.Sprintf("/courses/%d/assignments?per_page=100", courseID)
	body, err := c.makeRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var assignments []CanvasAssignment
	if err := json.Unmarshal(body, &assignments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal assignments: %w", err)
	}

	return assignments, nil
}

func (c *CanvasClient) GetSubmission(courseID, assignmentID, userID int) (*CanvasSubmission, error) {
	endpoint := fmt.Sprintf("/courses/%d/assignments/%d/submissions/%d", courseID, assignmentID, userID)
	body, err := c.makeRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var submission CanvasSubmission
	if err := json.Unmarshal(body, &submission); err != nil {
		return nil, fmt.Errorf("failed to unmarshal submission: %w", err)
	}

	return &submission, nil
}

func (c *CanvasClient) GetUpcomingAssignments(userID int) ([]CanvasAssignment, error) {
	courses, err := c.GetCourses()
	if err != nil {
		return nil, fmt.Errorf("failed to get courses: %w", err)
	}

	var allAssignments []CanvasAssignment
	twoWeeksFromNow := time.Now().AddDate(0, 0, 14)

	for _, course := range courses {
		assignments, err := c.GetAssignments(course.ID)
		if err != nil {
			fmt.Printf("Warning: failed to get assignments for course %s: %v\n", course.Name, err)
			continue
		}

		// Filter assignments due within 2 weeks
		for _, assignment := range assignments {
			if assignment.DueAt == "" {
				continue // Skip assignments with no due date
			}

			dueDate, err := time.Parse(time.RFC3339, assignment.DueAt)
			if err != nil {
				fmt.Printf("Warning: failed to parse due date for assignment %s: %v\n", assignment.Name, err)
				continue
			}

			// Only include assignments due within the next 2 weeks
			if dueDate.Before(twoWeeksFromNow) && dueDate.After(time.Now().AddDate(0, 0, -1)) {
				allAssignments = append(allAssignments, assignment)
			}
		}
	}

	return allAssignments, nil
}

func (c *CanvasClient) GetCourseNameByID(courseID int) (string, error) {
	courses, err := c.GetCourses()
	if err != nil {
		return "", err
	}

	for _, course := range courses {
		if course.ID == courseID {
			return course.Name, nil
		}
	}

	return fmt.Sprintf("Course %d", courseID), nil
}

func formatCanvasMetadata(assignment CanvasAssignment, courseName string, submission *CanvasSubmission) string {
	var grade string
	if submission != nil && submission.Score != nil {
		grade = fmt.Sprintf("%.1f%%", *submission.Score)
		if *submission.Score < 90 {
			grade += " (REDO NEEDED)"
		}
	} else {
		grade = "Not graded"
	}

	return fmt.Sprintf("\n\n---\nCanvas Assignment ID: %d\nCourse: %s\nOriginal Due Date: %s\nGrade: %s\nCanvas URL: %s",
		assignment.ID,
		courseName,
		assignment.DueAt,
		grade,
		assignment.HTMLURL)
}

func stripCanvasMetadata(description string) string {
	parts := strings.Split(description, "\n\n---\n")
	if len(parts) > 1 {
		return parts[0]
	}
	return description
}