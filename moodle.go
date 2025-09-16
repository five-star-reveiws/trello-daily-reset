package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "sort"
    "strings"
    "time"
)

// MoodleClient talks to Moodle/Open LMS Mobile App web services.
// Requires a token for service "moodle_mobile_app".
type MoodleClient struct {
    BaseURL string
    Token   string
}

type moodleSiteInfo struct {
    UserID int `json:"userid"`
    // Other fields omitted
}

type MoodleCourse struct {
    ID       int    `json:"id"`
    FullName string `json:"fullname"`
    ShortName string `json:"shortname"`
}

type MoodleAssignment struct {
    ID          int    `json:"id"`
    Name        string `json:"name"`
    Intro       string `json:"intro"`
    CourseID    int    `json:"course"`
    DueDateUnix int64  `json:"duedate"`
    URL         string `json:"url"`
}

type MoodleGrade struct {
    Grade      float64 `json:"grade"`
    GradeMax   float64 `json:"grademax"`
    UserID     int     `json:"userid"`
    ItemID     int     `json:"itemid"`
    Percentage float64 // Calculated field
}

type moodleAssignmentsResponse struct {
    Courses []struct {
        ID          int                 `json:"id"`
        FullName    string              `json:"fullname"`
        Assignments []MoodleAssignment  `json:"assignments"`
    } `json:"courses"`
    Warnings []any `json:"warnings"`
}

func NewMoodleClient(baseURL, token string) *MoodleClient {
    return &MoodleClient{BaseURL: strings.TrimRight(baseURL, "/"), Token: token}
}

func (m *MoodleClient) makeRequest(wsfunction string, params url.Values) ([]byte, error) {
    if params == nil {
        params = url.Values{}
    }
    params.Set("wstoken", m.Token)
    params.Set("wsfunction", wsfunction)
    params.Set("moodlewsrestformat", "json")

    endpoint := m.BaseURL + "/webservice/rest/server.php?" + params.Encode()

    resp, err := http.Get(endpoint)
    if err != nil {
        return nil, fmt.Errorf("moodle request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("moodle request status %d", resp.StatusCode)
    }
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("read moodle response: %w", err)
    }
    // Basic error envelope check
    if strings.Contains(string(body), "exception") && strings.Contains(string(body), "errorcode") {
        return nil, fmt.Errorf("moodle error: %s", string(body))
    }
    return body, nil
}

func (m *MoodleClient) GetSiteInfo() (int, error) {
    body, err := m.makeRequest("core_webservice_get_site_info", nil)
    if err != nil {
        return 0, err
    }
    var info moodleSiteInfo
    if err := json.Unmarshal(body, &info); err != nil {
        return 0, fmt.Errorf("decode site info: %w", err)
    }
    return info.UserID, nil
}

func (m *MoodleClient) GetCourses(userID int) ([]MoodleCourse, error) {
    params := url.Values{}
    params.Set("userid", fmt.Sprintf("%d", userID))
    body, err := m.makeRequest("core_enrol_get_users_courses", params)
    if err != nil {
        return nil, err
    }
    var courses []MoodleCourse
    if err := json.Unmarshal(body, &courses); err != nil {
        return nil, fmt.Errorf("decode courses: %w", err)
    }
    return courses, nil
}

func (m *MoodleClient) GetAssignments(courseIDs []int) ([]MoodleAssignment, map[int]string, error) {
    if len(courseIDs) == 0 {
        return nil, nil, nil
    }
    params := url.Values{}
    for i, id := range courseIDs {
        params.Set(fmt.Sprintf("courseids[%d]", i), fmt.Sprintf("%d", id))
    }
    body, err := m.makeRequest("mod_assign_get_assignments", params)
    if err != nil {
        return nil, nil, err
    }
    var resp moodleAssignmentsResponse
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, nil, fmt.Errorf("decode assignments: %w", err)
    }
    var out []MoodleAssignment
    courseNames := make(map[int]string)
    for _, c := range resp.Courses {
        courseNames[c.ID] = c.FullName
        for _, a := range c.Assignments {
            a.CourseID = c.ID // ensure set from container
            out = append(out, a)
        }
    }
    // stable order by duedate
    sort.Slice(out, func(i, j int) bool { return out[i].DueDateUnix < out[j].DueDateUnix })
    return out, courseNames, nil
}

// GetUpcomingAssignments returns assignments with due dates between now and toDate.
func (m *MoodleClient) GetUpcomingAssignments(toDate time.Time) ([]MoodleAssignment, map[int]string, error) {
    userID, err := m.GetSiteInfo()
    if err != nil {
        return nil, nil, err
    }
    courses, err := m.GetCourses(userID)
    if err != nil {
        return nil, nil, err
    }
    var courseIDs []int
    for _, c := range courses {
        courseIDs = append(courseIDs, c.ID)
    }
    all, names, err := m.GetAssignments(courseIDs)
    if err != nil {
        return nil, nil, err
    }
    now := time.Now()
    var filtered []MoodleAssignment
    for _, a := range all {
        if a.DueDateUnix == 0 {
            continue
        }
        due := time.Unix(a.DueDateUnix, 0)
        if due.After(now.Add(-24*time.Hour)) && due.Before(toDate.Add(24*time.Hour)) {
            filtered = append(filtered, a)
        }
    }
    return filtered, names, nil
}

// GetAssignmentGrade gets the grade for a specific assignment
func (m *MoodleClient) GetAssignmentGrade(assignmentID, userID int) (*MoodleGrade, error) {
    endpoint := fmt.Sprintf("%s/webservice/rest/server.php", m.BaseURL)

    params := url.Values{}
    params.Set("wstoken", m.Token)
    params.Set("wsfunction", "core_grades_get_grades")
    params.Set("moodlewsrestformat", "json")
    params.Set("courseid", fmt.Sprintf("%d", assignmentID)) // This might need adjustment based on Moodle API
    params.Set("userid", fmt.Sprintf("%d", userID))

    resp, err := http.Get(endpoint + "?" + params.Encode())
    if err != nil {
        return nil, fmt.Errorf("failed to get assignment grade: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API request failed with status: %s", resp.Status)
    }

    _, err = io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    // For now, return nil to indicate no grade available
    // This would need proper implementation based on Moodle's grade API structure
    return nil, nil
}

func formatMoodleMetadata(a MoodleAssignment, courseName string, grade *MoodleGrade) string {
    var due string
    if a.DueDateUnix > 0 {
        due = time.Unix(a.DueDateUnix, 0).Format(time.RFC3339)
    } else {
        due = ""
    }

    var gradeStr string
    if grade != nil && grade.GradeMax > 0 {
        percentage := (grade.Grade / grade.GradeMax) * 100
        gradeStr = fmt.Sprintf("%.1f%%", percentage)
        if percentage < 90 {
            gradeStr += " (REDO NEEDED)"
        }
    } else {
        gradeStr = "Not graded"
    }

    return fmt.Sprintf("\n\n---\nMoodle Assignment ID: %d\nCourse: %s\nOriginal Due Date: %s\nGrade: %s\nMoodle URL: %s",
        a.ID, courseName, due, gradeStr, a.URL)
}

