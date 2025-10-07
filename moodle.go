package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
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
    Type        string // "assignment" or "quiz"
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

type moodleQuiz struct {
    ID          int    `json:"id"`
    Name        string `json:"name"`
    Intro       string `json:"intro"`
    CourseID    int    `json:"course"`
    TimeClose   int64  `json:"timeclose"`
    URL         string `json:"url"`
}

type moodleQuizzesResponse struct {
    Quizzes  []moodleQuiz `json:"quizzes"`
    Warnings []any        `json:"warnings"`
}

func NewMoodleClient(baseURL, token string) *MoodleClient {
    return &MoodleClient{BaseURL: strings.TrimRight(baseURL, "/"), Token: token}
}

type MoodleTestData struct {
    Assignments []MoodleAssignment `json:"assignments"`
    CourseNames map[int]string     `json:"course_names"`
    Grades      map[int]*MoodleGrade `json:"grades"` // key is assignment ID
}

func (m *MoodleClient) LoadTestData(filename string) (*MoodleTestData, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("failed to read test file: %w", err)
    }

    var testData MoodleTestData
    if err := json.Unmarshal(data, &testData); err != nil {
        return nil, fmt.Errorf("failed to parse test data: %w", err)
    }

    return &testData, nil
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
            a.Type = "assignment"
            out = append(out, a)
        }
    }
    // stable order by duedate
    sort.Slice(out, func(i, j int) bool { return out[i].DueDateUnix < out[j].DueDateUnix })
    return out, courseNames, nil
}

func (m *MoodleClient) GetQuizzes(courseIDs []int) ([]MoodleAssignment, map[int]string, error) {
    if len(courseIDs) == 0 {
        return nil, nil, nil
    }
    params := url.Values{}
    for i, id := range courseIDs {
        params.Set(fmt.Sprintf("courseids[%d]", i), fmt.Sprintf("%d", id))
    }
    body, err := m.makeRequest("mod_quiz_get_quizzes_by_courses", params)
    if err != nil {
        return nil, nil, err
    }
    var resp moodleQuizzesResponse
    if err := json.Unmarshal(body, &resp); err != nil {
        return nil, nil, fmt.Errorf("decode quizzes: %w", err)
    }
    var out []MoodleAssignment
    courseNames := make(map[int]string)

    // Group quizzes by course
    quizzesByCourse := make(map[int][]moodleQuiz)
    for _, quiz := range resp.Quizzes {
        quizzesByCourse[quiz.CourseID] = append(quizzesByCourse[quiz.CourseID], quiz)
    }

    // Get course names by fetching course info
    userID, err := m.GetSiteInfo()
    if err == nil {
        courses, err := m.GetCourses(userID)
        if err == nil {
            for _, c := range courses {
                courseNames[c.ID] = c.FullName
            }
        }
    }

    // Convert quizzes to assignments
    for courseID, quizzes := range quizzesByCourse {
        for _, quiz := range quizzes {
            assignment := MoodleAssignment{
                ID:          quiz.ID,
                Name:        quiz.Name,
                Intro:       quiz.Intro,
                CourseID:    courseID,
                DueDateUnix: quiz.TimeClose, // Use timeclose as due date
                URL:         quiz.URL,
                Type:        "quiz",
            }
            out = append(out, assignment)
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
    // Get assignments
    assignments, assignmentNames, err := m.GetAssignments(courseIDs)
    if err != nil {
        return nil, nil, err
    }

    // Get quizzes
    quizzes, quizNames, err := m.GetQuizzes(courseIDs)
    if err != nil {
        fmt.Printf("Warning: failed to get quizzes: %v\n", err)
        quizzes = nil
        quizNames = make(map[int]string)
    }

    // Merge assignments and quizzes
    all := append(assignments, quizzes...)

    // Merge course names (quiz names take precedence if different)
    names := assignmentNames
    for k, v := range quizNames {
        names[k] = v
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

// GetAssignmentGrade gets the grade for a specific assignment or quiz
func (m *MoodleClient) GetAssignmentGrade(assignmentID, courseID, userID int, activityType string) (*MoodleGrade, error) {
    var wsfunction string

    // Use different API functions based on activity type
    if activityType == "quiz" {
        wsfunction = "mod_quiz_get_user_attempts"
    } else {
        wsfunction = "mod_assign_get_submissions"
    }

    params := url.Values{}
    params.Set("wstoken", m.Token)
    params.Set("wsfunction", wsfunction)
    params.Set("moodlewsrestformat", "json")

    if activityType == "quiz" {
        params.Set("quizid", fmt.Sprintf("%d", assignmentID))
        params.Set("userid", fmt.Sprintf("%d", userID))
    } else {
        params.Set("assignmentids[0]", fmt.Sprintf("%d", assignmentID))
    }

    body, err := m.makeRequest(wsfunction, params)
    if err != nil {
        return nil, fmt.Errorf("failed to get grade for %s %d: %w", activityType, assignmentID, err)
    }

    if activityType == "quiz" {
        return m.parseQuizGrade(body, userID)
    } else {
        return m.parseAssignmentGrade(body, userID)
    }
}

func (m *MoodleClient) parseQuizGrade(body []byte, userID int) (*MoodleGrade, error) {
    var response struct {
        Attempts []struct {
            UserID int     `json:"userid"`
            Sumgrades *float64 `json:"sumgrades"`
            State  string  `json:"state"`
            Quiz   json.RawMessage `json:"quiz"` // Use RawMessage to handle variable structure
        } `json:"attempts"`
    }

    if err := json.Unmarshal(body, &response); err != nil {
        // If parsing fails, try to get more info from the response
        fmt.Printf("Debug: Quiz API response: %s\n", string(body))
        return nil, nil // Return nil instead of error to avoid breaking sync
    }

    // Find the latest attempt for this user
    for _, attempt := range response.Attempts {
        if attempt.UserID == userID && attempt.State == "finished" && attempt.Sumgrades != nil {
            // For now, assume 100 as max grade if we can't parse quiz structure
            maxGrade := 100.0

            // Try to parse quiz structure if available
            if len(attempt.Quiz) > 0 {
                var quizInfo struct {
                    Sumgrades float64 `json:"sumgrades"`
                }
                if err := json.Unmarshal(attempt.Quiz, &quizInfo); err == nil {
                    maxGrade = quizInfo.Sumgrades
                }
            }

            grade := &MoodleGrade{
                Grade:      *attempt.Sumgrades,
                GradeMax:   maxGrade,
                UserID:     userID,
                Percentage: (*attempt.Sumgrades / maxGrade) * 100,
            }
            return grade, nil
        }
    }

    return nil, nil // No grade found
}

func (m *MoodleClient) parseAssignmentGrade(body []byte, userID int) (*MoodleGrade, error) {
    var response struct {
        Assignments []struct {
            Submissions []struct {
                UserID int      `json:"userid"`
                Grade  *string `json:"grade"`
                Status string  `json:"status"`
                Assignment struct {
                    Grade float64 `json:"grade"`
                } `json:"assignment"`
            } `json:"submissions"`
        } `json:"assignments"`
    }

    if err := json.Unmarshal(body, &response); err != nil {
        return nil, fmt.Errorf("failed to parse assignment submissions: %w", err)
    }

    // Find submission for this user
    for _, assignment := range response.Assignments {
        for _, submission := range assignment.Submissions {
            if submission.UserID == userID && submission.Grade != nil {
                gradeValue := 0.0
                if submission.Grade != nil {
                    // Parse grade (might be numeric or percentage)
                    if strings.HasSuffix(*submission.Grade, "%") {
                        fmt.Sscanf(*submission.Grade, "%f%%", &gradeValue)
                    } else {
                        fmt.Sscanf(*submission.Grade, "%f", &gradeValue)
                    }
                }

                grade := &MoodleGrade{
                    Grade:      gradeValue,
                    GradeMax:   submission.Assignment.Grade,
                    UserID:     userID,
                    Percentage: (gradeValue / submission.Assignment.Grade) * 100,
                }
                return grade, nil
            }
        }
    }

    return nil, nil // No grade found
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

    activityType := "Assignment"
    if a.Type == "quiz" {
        activityType = "Quiz"
    }

    return fmt.Sprintf("\n\n---\nMoodle %s ID: %d\nCourse: %s\nOriginal Due Date: %s\nGrade: %s\nMoodle URL: %s",
        activityType, a.ID, courseName, due, gradeStr, a.URL)
}

