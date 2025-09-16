package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type TrelloClient struct {
	APIKey   string
	APIToken string
	BaseURL  string
}

type Card struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"desc"`
	URL         string    `json:"url"`
	ShortURL    string    `json:"shortUrl"`
	Closed      bool      `json:"closed"`
	IDList      string    `json:"idList"`
	Due         *time.Time `json:"due"`
	DueComplete bool      `json:"dueComplete"`
}

type Board struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type List struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	BoardID string `json:"idBoard"`
}

type CachedData struct {
	Boards []Board `json:"boards"`
	Lists  []List  `json:"lists"`
}

func NewTrelloClient(apiKey, apiToken string) *TrelloClient {
	return &TrelloClient{
		APIKey:   apiKey,
		APIToken: apiToken,
		BaseURL:  "https://api.trello.com/1",
	}
}

func (c *TrelloClient) makeRequest(endpoint string) ([]byte, error) {
	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("token", c.APIToken)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

func (c *TrelloClient) GetBoards() ([]Board, error) {
	endpoint := "/members/me/boards"

	body, err := c.makeRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var boards []Board
	if err := json.Unmarshal(body, &boards); err != nil {
		return nil, fmt.Errorf("failed to unmarshal boards: %w", err)
	}

	return boards, nil
}

func (c *TrelloClient) GetListsInBoard(boardID string) ([]List, error) {
	endpoint := fmt.Sprintf("/boards/%s/lists", boardID)

	body, err := c.makeRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var lists []List
	if err := json.Unmarshal(body, &lists); err != nil {
		return nil, fmt.Errorf("failed to unmarshal lists: %w", err)
	}

	return lists, nil
}

// GetBoardLists is an alias for GetListsInBoard for consistency
func (c *TrelloClient) GetBoardLists(boardID string) ([]List, error) {
	return c.GetListsInBoard(boardID)
}

func (c *TrelloClient) GetCardsInList(listID string) ([]Card, error) {
	endpoint := fmt.Sprintf("/lists/%s/cards", listID)

	body, err := c.makeRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var cards []Card
	if err := json.Unmarshal(body, &cards); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cards: %w", err)
	}

	return cards, nil
}

func (c *TrelloClient) CacheData() error {
	boards, err := c.GetBoards()
	if err != nil {
		return fmt.Errorf("failed to get boards: %w", err)
	}

	var allLists []List
	for _, board := range boards {
		lists, err := c.GetListsInBoard(board.ID)
		if err != nil {
			return fmt.Errorf("failed to get lists for board %s: %w", board.Name, err)
		}
		allLists = append(allLists, lists...)
	}

	cache := CachedData{
		Boards: boards,
		Lists:  allLists,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	return os.WriteFile("trello_cache.json", data, 0644)
}

func (c *TrelloClient) LoadCache() (*CachedData, error) {
	data, err := os.ReadFile("trello_cache.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache CachedData
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	return &cache, nil
}

func (c *TrelloClient) UpdateCard(cardID, due string, dueComplete bool) error {
	endpoint := fmt.Sprintf("/cards/%s", cardID)

	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("token", c.APIToken)
	q.Set("due", due)
	q.Set("dueComplete", fmt.Sprintf("%t", dueComplete))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("PUT", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *TrelloClient) ResetDailyTasks(boardName, listName string) error {
	listID, err := c.FindListByName(boardName, listName)
	if err != nil {
		return err
	}

	cards, err := c.GetCardsInList(listID)
	if err != nil {
		return fmt.Errorf("failed to get cards: %w", err)
	}

	// Calculate next day due date (end of tomorrow)
	tomorrow := time.Now().AddDate(0, 0, 1)
	endOfTomorrow := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 23, 59, 59, 0, tomorrow.Location())
	dueDate := endOfTomorrow.Format("2006-01-02T15:04:05.000Z")

	fmt.Printf("Resetting %d daily tasks with due date: %s\n", len(cards), endOfTomorrow.Format("Jan 2, 2006 3:04 PM"))

	for _, card := range cards {
		fmt.Printf("Updating: %s\n", card.Name)
		if err := c.UpdateCard(card.ID, dueDate, false); err != nil {
			return fmt.Errorf("failed to update card %s: %w", card.Name, err)
		}
	}

	fmt.Printf("Successfully reset %d daily tasks!\n", len(cards))
	return nil
}

func (c *TrelloClient) CreateCard(listID, name, desc, due string) error {
	endpoint := "/cards"

	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("token", c.APIToken)
	q.Set("idList", listID)
	q.Set("name", name)
	if desc != "" {
		q.Set("desc", desc)
	}
	if due != "" {
		q.Set("due", due)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	return nil
}

func (c *TrelloClient) CreateWeeklyCards() error {
	// Load subjects configuration
	config, err := LoadSubjectsConfig()
	if err != nil {
		return fmt.Errorf("failed to load subjects config: %w", err)
	}

	// Get current quarter and week
	quarter, err := config.GetCurrentQuarter()
	if err != nil {
		return fmt.Errorf("failed to get current quarter: %w", err)
	}

	currentWeek, err := quarter.GetCurrentWeek()
	if err != nil {
		return fmt.Errorf("failed to get current week: %w", err)
	}

	// Get next week
	nextWeek, err := quarter.GetNextWeek(currentWeek)
	if err != nil {
		return fmt.Errorf("failed to get next week: %w", err)
	}

	// Get the Weekly list ID
	listID, err := c.FindListByName("Makai School", "Weekly")
	if err != nil {
		return fmt.Errorf("failed to find Weekly list: %w", err)
	}

	// Calculate due date (end of week at 6 PM)
	endDate, err := time.Parse("2006-01-02", nextWeek.EndDate)
	if err != nil {
		return fmt.Errorf("failed to parse end date: %w", err)
	}
	dueTime := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 18, 0, 0, 0, endDate.Location())
	dueDate := dueTime.Format("2006-01-02T15:04:05.000Z")

	// Format week range
	weekRange := quarter.FormatWeekRange(nextWeek)

	fmt.Printf("Creating cards for Week %d: %s\n", nextWeek.Number, weekRange)
	fmt.Printf("Due date: %s\n", dueTime.Format("January 2, 2006 at 3:04 PM"))

	// Create cards for each subject
	for _, subject := range quarter.Subjects {
		cardName := fmt.Sprintf("%s Week %d: %s", subject, nextWeek.Number, weekRange)

		fmt.Printf("Creating: %s\n", cardName)
		if err := c.CreateCard(listID, cardName, "", dueDate); err != nil {
			return fmt.Errorf("failed to create card for %s: %w", subject, err)
		}
	}

	fmt.Printf("Successfully created %d weekly cards!\n", len(quarter.Subjects))
	return nil
}

func (c *TrelloClient) GetAllBoardCards(boardName string) ([]Card, error) {
	// First find the board ID
	cache, err := c.LoadCache()
	if err != nil {
		return nil, fmt.Errorf("failed to load cache: %w", err)
	}

	var boardID string
	for _, board := range cache.Boards {
		if normalizeString(board.Name) == normalizeString(boardName) {
			boardID = board.ID
			break
		}
	}

	if boardID == "" {
		return nil, fmt.Errorf("board '%s' not found", boardName)
	}

	// Get all cards from the board
	endpoint := fmt.Sprintf("/boards/%s/cards", boardID)
	body, err := c.makeRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var cards []Card
	if err := json.Unmarshal(body, &cards); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cards: %w", err)
	}

	return cards, nil
}

func (c *TrelloClient) FindCardByCanvasID(cards []Card, canvasID int, canvasType string) *Card {
    searchPattern := fmt.Sprintf("Canvas %s ID: %d", canvasType, canvasID)

    for i, card := range cards {
        if strings.Contains(card.Description, searchPattern) {
            return &cards[i]
        }
    }

    return nil
}

func (c *TrelloClient) FindCardByMoodleAssignmentID(cards []Card, moodleID int) *Card {
    searchPattern := fmt.Sprintf("Moodle Assignment ID: %d", moodleID)

    for i, card := range cards {
        if strings.Contains(card.Description, searchPattern) {
            return &cards[i]
        }
    }
    return nil
}


func (c *TrelloClient) SortCardsByDueDate(listID string) error {
	// Get all cards in the list
	cards, err := c.GetCardsInList(listID)
	if err != nil {
		return fmt.Errorf("failed to get cards: %w", err)
	}

	if len(cards) <= 1 {
		return nil // No need to sort
	}

	// Sort cards by due date (cards without due dates go to the end)
	sort.Slice(cards, func(i, j int) bool {
		cardI, cardJ := cards[i], cards[j]

		// Cards without due dates go to the end
		if cardI.Due == nil && cardJ.Due == nil {
			return false // Preserve existing order for cards without due dates
		}
		if cardI.Due == nil {
			return false // cardI goes after cardJ
		}
		if cardJ.Due == nil {
			return true // cardI goes before cardJ
		}

		// Both have due dates - sort by earliest first
		return cardI.Due.Before(*cardJ.Due)
	})

	// Update card positions in Trello - move cards in reverse order
	// so the first card (earliest due date) ends up at the top
	for i := len(cards) - 1; i >= 0; i-- {
		card := cards[i]
		err := c.UpdateCardPosition(card.ID, "top")
		if err != nil {
			fmt.Printf("Warning: failed to update position for card %s: %v\n", card.Name, err)
		}
		// Small delay to avoid rate limiting
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	fmt.Printf("✅ Sorted %d cards by due date in list\n", len(cards))
	return nil
}

func (c *TrelloClient) UpdateCardPosition(cardID, position string) error {
	endpoint := fmt.Sprintf("/cards/%s", cardID)

	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("token", c.APIToken)
	q.Set("pos", position)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("PUT", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update card position: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return nil
}

func (c *TrelloClient) UpdateCardDescription(cardID, description string) error {
	endpoint := fmt.Sprintf("/cards/%s", cardID)

	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("token", c.APIToken)
	q.Set("desc", description)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("PUT", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return nil
}

func (c *TrelloClient) SyncCanvasAssignments(canvasClient *CanvasClient, canvasUserID int) error {
	fmt.Println("Starting Canvas sync...")

	// Get upcoming assignments from Canvas
	assignments, err := canvasClient.GetUpcomingAssignments(canvasUserID)
	if err != nil {
		return fmt.Errorf("failed to get Canvas assignments: %w", err)
	}

	fmt.Printf("Found %d assignments due within 3 months\n", len(assignments))

	// Get all cards from the Makai School board
	allCards, err := c.GetAllBoardCards("Makai School")
	if err != nil {
		return fmt.Errorf("failed to get Trello cards: %w", err)
	}

	fmt.Printf("Found %d existing cards on Makai School board\n", len(allCards))

	// Get the Weekly list ID for new cards
	weeklyListID, err := c.FindListByName("Makai School", "Weekly")
	if err != nil {
		return fmt.Errorf("failed to find Weekly list: %w", err)
	}

	// Process each Canvas assignment
	for _, assignment := range assignments {
		courseName, err := canvasClient.GetCourseNameByID(assignment.CourseID)
		if err != nil {
			fmt.Printf("Warning: failed to get course name for %d: %v\n", assignment.CourseID, err)
			courseName = fmt.Sprintf("Course %d", assignment.CourseID)
		}

		// Get grade/submission info
		submission, err := canvasClient.GetSubmission(assignment.CourseID, assignment.ID, canvasUserID)
		if err != nil {
			fmt.Printf("Warning: failed to get submission for assignment %s: %v\n", assignment.Name, err)
			submission = nil
		}

		// Check if card already exists
		existingCard := c.FindCardByCanvasID(allCards, assignment.ID, "Assignment")

		// Prepare card data
		cardTitle := fmt.Sprintf("%s - %s", courseName, assignment.Name)
		needsRedo := submission != nil && submission.Score != nil && *submission.Score < 90
		if needsRedo && !strings.HasPrefix(cardTitle, "REDO - ") {
			cardTitle = "REDO - " + cardTitle
		} else if !needsRedo && strings.HasPrefix(cardTitle, "REDO - ") {
			cardTitle = strings.TrimPrefix(cardTitle, "REDO - ")
		}

		// Prepare description with Canvas metadata
		baseDescription := stripCanvasMetadata(assignment.Description)
		canvasMetadata := formatCanvasMetadata(assignment, courseName, submission)
		fullDescription := baseDescription + canvasMetadata

		// Calculate due date (use Canvas due date, or 1 week from now for REDO)
		var dueDate string
		if needsRedo {
			redoDate := time.Now().AddDate(0, 0, 7)
			dueDate = redoDate.Format("2006-01-02T15:04:05.000Z")
		} else if assignment.DueAt != "" {
			// Convert Canvas date to Trello format
			canvasDue, err := time.Parse(time.RFC3339, assignment.DueAt)
			if err == nil {
				dueDate = canvasDue.Format("2006-01-02T15:04:05.000Z")
			}
		}

		if existingCard != nil {
			// Update existing card
			fmt.Printf("Updating existing card: %s\n", cardTitle)
			if err := c.UpdateCard(existingCard.ID, dueDate, false); err != nil {
				fmt.Printf("Warning: failed to update due date for card %s: %v\n", cardTitle, err)
			}
			// Note: We'd need a UpdateCardNameAndDescription function for full updates
		} else {
			// Create new card
			fmt.Printf("Creating new card: %s\n", cardTitle)
			if err := c.CreateCard(weeklyListID, cardTitle, fullDescription, dueDate); err != nil {
				fmt.Printf("Warning: failed to create card %s: %v\n", cardTitle, err)
			}
		}
	}

	fmt.Printf("Canvas sync completed successfully!\n")

	// Sort cards by due date in the Weekly list
	fmt.Println("Sorting cards by due date...")
	if err := c.SortCardsByDueDate(weeklyListID); err != nil {
		fmt.Printf("Warning: failed to sort cards by due date: %v\n", err)
	}

	return nil
}


func (c *TrelloClient) SyncMoodleAssignments(moodleClient *MoodleClient, toDate time.Time, dryRun bool) error {
    fmt.Println("Starting Moodle/Open LMS sync...")

    // Pull upcoming assignments
    assignments, courseNames, err := moodleClient.GetUpcomingAssignments(toDate)
    if err != nil {
        return fmt.Errorf("failed to get Moodle assignments: %w", err)
    }
    fmt.Printf("Found %d Moodle assignments due by %s\n", len(assignments), toDate.Format("2006-01-02"))

    // Get all cards from the Makai School board
    allCards, err := c.GetAllBoardCards("Makai School")
    if err != nil {
        return fmt.Errorf("failed to get Trello cards: %w", err)
    }
    fmt.Printf("Found %d existing cards on Makai School board\n", len(allCards))

    var weeklyListID string
    if !dryRun {
        // Weekly list for new cards
        var err error
        weeklyListID, err = c.FindListByName("Makai School", "Weekly")
        if err != nil {
            return fmt.Errorf("failed to find Weekly list: %w", err)
        }
    }

    for _, a := range assignments {
        courseName := courseNames[a.CourseID]
        if courseName == "" {
            courseName = fmt.Sprintf("Course %d", a.CourseID)
        }

        // Get grade for this assignment (placeholder - will return nil for now)
        var grade *MoodleGrade
        // TODO: Implement actual grade checking when Moodle API details are available
        // grade, err := moodleClient.GetAssignmentGrade(a.ID, userID)
        // if err != nil {
        //     fmt.Printf("Warning: failed to get grade for assignment %s: %v\n", a.Name, err)
        // }

        // Check if assignment has passing grade (>= 90%) and skip if so
        if grade != nil && grade.GradeMax > 0 {
            percentage := (grade.Grade / grade.GradeMax) * 100
            if percentage >= 90 {
                fmt.Printf("Skipping assignment with passing grade: %s (%.1f%%)\n", a.Name, percentage)
                continue
            }
        }

        cardTitle := fmt.Sprintf("%s - %s", courseName, a.Name)

        // Add REDO prefix if grade is below 90%
        needsRedo := grade != nil && grade.GradeMax > 0 && (grade.Grade/grade.GradeMax)*100 < 90
        if needsRedo && !strings.HasPrefix(cardTitle, "REDO - ") {
            cardTitle = "REDO - " + cardTitle
        } else if !needsRedo && strings.HasPrefix(cardTitle, "REDO - ") {
            cardTitle = strings.TrimPrefix(cardTitle, "REDO - ")
        }

        baseDescription := a.Intro
        // Many Moodle sites return HTML in Intro; keep as-is to preserve formatting.
        meta := formatMoodleMetadata(a, courseName, grade)
        fullDescription := strings.TrimSpace(baseDescription) + meta

        // Due date
        var dueDate string
        if a.DueDateUnix > 0 {
            due := time.Unix(a.DueDateUnix, 0)
            dueDate = due.Format("2006-01-02T15:04:05.000Z")
        }

        // Check for existing card
        existing := c.FindCardByMoodleAssignmentID(allCards, a.ID)
        if existing != nil {
            if dryRun {
                fmt.Printf("[DRY RUN] Would update card: %s (due %s)\n", cardTitle, dueDate)
            } else {
                fmt.Printf("Updating existing Moodle card: %s\n", cardTitle)
                if err := c.UpdateCard(existing.ID, dueDate, false); err != nil {
                    fmt.Printf("Warning: failed to update due date for %s: %v\n", cardTitle, err)
                }
                if existing.Description != fullDescription {
                    if err := c.UpdateCardDescription(existing.ID, fullDescription); err != nil {
                        fmt.Printf("Warning: failed to update description for %s: %v\n", cardTitle, err)
                    }
                }
            }
        } else {
            if dryRun {
                fmt.Printf("[DRY RUN] Would create card: %s (due %s)\n", cardTitle, dueDate)
            } else {
                fmt.Printf("Creating new Moodle card: %s\n", cardTitle)
                if err := c.CreateCard(weeklyListID, cardTitle, fullDescription, dueDate); err != nil {
                    fmt.Printf("Warning: failed to create card %s: %v\n", cardTitle, err)
                }
            }
        }
    }

    fmt.Printf("Moodle sync completed successfully!\n")

    // Sort cards by due date in the Weekly list (if not dry run)
    if !dryRun {
        fmt.Println("Sorting cards by due date...")
        if err := c.SortCardsByDueDate(weeklyListID); err != nil {
            fmt.Printf("Warning: failed to sort cards by due date: %v\n", err)
        }
    }

    return nil
}

// JiraTask represents a JIRA task parsed from local files
type JiraTask struct {
	ID          string
	Title       string
	Status      string
	NextSteps   string
	KeyFindings string
	JiraStatus  string
	Priority    string
	IssueType   string
	PRLink      string
}

// SyncJiraTasks syncs local JIRA tasks to Trello Mac board
func (c *TrelloClient) SyncJiraTasks(tasksDir string) error {
	fmt.Printf("Syncing JIRA tasks from %s\n", tasksDir)

	// Get Mac board
	boards, err := c.GetBoards()
	if err != nil {
		return fmt.Errorf("failed to get boards: %v", err)
	}

	var macBoardID string
	for _, board := range boards {
		if board.Name == "Mac" {
			macBoardID = board.ID
			break
		}
	}

	if macBoardID == "" {
		return fmt.Errorf("Mac board not found")
	}

	// Get board lists and cards
	lists, err := c.GetBoardLists(macBoardID)
	if err != nil {
		return fmt.Errorf("failed to get board lists: %v", err)
	}

	cards, err := c.GetAllBoardCards("Mac")
	if err != nil {
		return fmt.Errorf("failed to get board cards: %v", err)
	}

	// Create list ID to name mapping
	listIDToName := make(map[string]string)
	for _, list := range lists {
		listIDToName[list.ID] = list.Name
	}

	// Use first list as default for new cards
	var defaultListID string
	if len(lists) > 0 {
		defaultListID = lists[0].ID
		fmt.Printf("Using list '%s' for new cards\n", lists[0].Name)
	} else {
		return fmt.Errorf("no lists found on Mac board")
	}

	// Parse JIRA tasks from directory
	tasks, err := c.parseJiraTasks(tasksDir)
	if err != nil {
		return fmt.Errorf("failed to parse JIRA tasks: %v", err)
	}

	fmt.Printf("Found %d JIRA tasks\n", len(tasks))

	// Process each task
	updatedCards := 0
	createdCards := 0

	for _, task := range tasks {
		fmt.Printf("Processing task: %s\n", task.ID)

		// Find matching card by task ID in title
		existingCard := c.FindCardByTaskID(cards, task.ID)

		if existingCard != nil {
			fmt.Printf("  Found existing card: %s\n", existingCard.Name)

			// Fix duplicate task ID in title if present (e.g., "AK-123: AK-123: Title")
			if strings.Count(existingCard.Name, task.ID+":") > 1 {
				fmt.Printf("  Fixing duplicate title\n")
				// Remove the first occurrence of "taskID: "
				fixedTitle := strings.Replace(existingCard.Name, task.ID+": ", "", 1)
				if err := c.UpdateCardTitle(existingCard.ID, fixedTitle); err != nil {
					fmt.Printf("  Warning: failed to fix card title: %v\n", err)
				} else {
					fmt.Printf("  ✓ Fixed duplicate title\n")
				}
			}

			// Update local status and JIRA based on Trello list position
			if listName, exists := listIDToName[existingCard.IDList]; exists {
				// Update local status
				newStatus := c.mapListNameToStatus(listName)
				if err := c.updateLocalTaskStatus(tasksDir, task.ID, newStatus); err != nil {
					fmt.Printf("  Warning: failed to update local status: %v\n", err)
				} else {
					fmt.Printf("  ✓ Updated local status to: %s (from %s list)\n", newStatus, listName)
				}

				// Update JIRA status
				jiraStatus := c.mapListNameToJiraStatus(listName)
				if jiraStatus != "" {
					if err := c.updateJiraStatus(task.ID, jiraStatus); err != nil {
						fmt.Printf("  Warning: failed to update JIRA status: %v\n", err)
					} else {
						fmt.Printf("  ✓ Updated JIRA status to: %s\n", jiraStatus)
					}
				}

			}

			// Update card description with current status
			description := c.buildJiraCardDescription(task)
			if err := c.UpdateCardDescription(existingCard.ID, description); err != nil {
				fmt.Printf("  Warning: failed to update card description: %v\n", err)
			} else {
				fmt.Printf("  ✓ Updated card description\n")
				updatedCards++
			}

			// Add red label for bugs (check both IssueType and Priority fields)
			isBug := strings.ToLower(task.IssueType) == "bug" || strings.ToLower(task.Priority) == "bug"
			if isBug {
				if err := c.AddLabelToCard(existingCard.ID, "red"); err != nil {
					fmt.Printf("  Warning: failed to add bug label: %v\n", err)
				} else {
					fmt.Printf("  ✓ Added bug label\n")
				}
			}
		} else {
			fmt.Printf("  Creating new card for task\n")

			// Create new card - only add task ID if title doesn't already contain it
			var cardTitle string
			if strings.HasPrefix(task.Title, task.ID+":") {
				cardTitle = task.Title
			} else {
				cardTitle = fmt.Sprintf("%s: %s", task.ID, task.Title)
			}
			description := c.buildJiraCardDescription(task)

			if err := c.CreateCard(defaultListID, cardTitle, description, ""); err != nil {
				fmt.Printf("  Warning: failed to create card: %v\n", err)
			} else {
				fmt.Printf("  ✓ Created new card\n")
				createdCards++

				// Add red label for bugs (need to get the card ID first)
				isBug := strings.ToLower(task.IssueType) == "bug" || strings.ToLower(task.Priority) == "bug"
				if isBug {
					// Find the newly created card to get its ID
					newCards, err := c.GetAllBoardCards("Mac")
					if err == nil {
						if newCard := c.FindCardByTaskID(newCards, task.ID); newCard != nil {
							if err := c.AddLabelToCard(newCard.ID, "red"); err != nil {
								fmt.Printf("  Warning: failed to add bug label: %v\n", err)
							} else {
								fmt.Printf("  ✓ Added bug label\n")
							}
						}
					}
				}
			}
		}
	}

	fmt.Printf("\nJIRA sync completed!\n")
	fmt.Printf("Created: %d cards\n", createdCards)
	fmt.Printf("Updated: %d cards\n", updatedCards)

	return nil
}

// FindCardByTaskID finds a card that contains the task ID in its title
func (c *TrelloClient) FindCardByTaskID(cards []Card, taskID string) *Card {
	for i := range cards {
		if strings.Contains(cards[i].Name, taskID) {
			return &cards[i]
		}
	}
	return nil
}

// parseJiraTasks reads and parses JIRA tasks from the directory
func (c *TrelloClient) parseJiraTasks(tasksDir string) ([]JiraTask, error) {
	var tasks []JiraTask

	entries, err := os.ReadDir(tasksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		taskID := entry.Name()
		statusFile := filepath.Join(tasksDir, taskID, "STATUS.md")
		taskFile := filepath.Join(tasksDir, taskID, taskID+".md")

		task, err := c.parseJiraTask(taskID, statusFile, taskFile)
		if err != nil {
			fmt.Printf("Warning: failed to parse task %s: %v\n", taskID, err)
			continue
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// parseJiraTask parses a single JIRA task from its files
func (c *TrelloClient) parseJiraTask(taskID, statusFile, taskFile string) (JiraTask, error) {
	task := JiraTask{ID: taskID}

	// Read STATUS.md file
	if statusData, err := os.ReadFile(statusFile); err == nil {
		statusContent := string(statusData)

		// Extract current status
		if match := regexp.MustCompile(`## Current Status:\s*(.+)`).FindStringSubmatch(statusContent); len(match) > 1 {
			task.Status = strings.TrimSpace(match[1])
		}

		// Extract next steps
		if match := regexp.MustCompile(`(?s)## Next Steps:(.*?)(?:## |$)`).FindStringSubmatch(statusContent); len(match) > 1 {
			task.NextSteps = strings.TrimSpace(match[1])
		}

		// Extract key findings
		if match := regexp.MustCompile(`(?s)## Key Findings:(.*?)(?:## |$)`).FindStringSubmatch(statusContent); len(match) > 1 {
			task.KeyFindings = strings.TrimSpace(match[1])
		}

		// Extract JIRA info
		if match := regexp.MustCompile(`- \*\*JIRA Status\*\*:\s*(.+)`).FindStringSubmatch(statusContent); len(match) > 1 {
			task.JiraStatus = strings.TrimSpace(match[1])
		}
		if match := regexp.MustCompile(`- \*\*Priority\*\*:\s*(.+)`).FindStringSubmatch(statusContent); len(match) > 1 {
			task.Priority = strings.TrimSpace(match[1])
		}
		if match := regexp.MustCompile(`- \*\*Issue Type\*\*:\s*(.+)`).FindStringSubmatch(statusContent); len(match) > 1 {
			task.IssueType = strings.TrimSpace(match[1])
		}

		// Extract PR link from Context Links section - try multiple patterns
		prPatterns := []string{
			`- 📋 \[Related PR\]\(([^)]+)\)`,              // Standard format
			`- 📋 \[PR\]\(([^)]+)\)`,                      // Short format
			`- \[PR\]\(([^)]+)\)`,                         // Without emoji
			`- \[Related PR\]\(([^)]+)\)`,                 // Without emoji
			`https://github\.com/[^\s)]+/pull/\d+`,       // Direct GitHub PR URLs
		}

		for _, pattern := range prPatterns {
			if match := regexp.MustCompile(pattern).FindStringSubmatch(statusContent); len(match) > 1 {
				prLink := strings.TrimSpace(match[1])
				if prLink != "#" && prLink != "" && prLink != "<!-- Add PR link when created -->" {
					task.PRLink = prLink
					break
				}
			} else if pattern == `https://github\.com/[^\s)]+/pull/\d+` {
				// Handle direct URL match
				if match := regexp.MustCompile(pattern).FindString(statusContent); match != "" {
					task.PRLink = match
					break
				}
			}
		}
	}

	// Read task.md file for title
	if taskData, err := os.ReadFile(taskFile); err == nil {
		taskContent := string(taskData)

		// Extract title from first heading
		if match := regexp.MustCompile(`# (.+)`).FindStringSubmatch(taskContent); len(match) > 1 {
			task.Title = strings.TrimSpace(match[1])
		}
	}

	// Default title if not found
	if task.Title == "" {
		task.Title = "JIRA Task"
	}

	return task, nil
}

// buildJiraCardDescription creates a description for the Trello card
func (c *TrelloClient) buildJiraCardDescription(task JiraTask) string {
	var desc strings.Builder

	desc.WriteString(fmt.Sprintf("**JIRA Task ID**: %s\n\n", task.ID))

	if task.Status != "" {
		desc.WriteString(fmt.Sprintf("**Current Status**: %s\n\n", task.Status))
	}

	if task.JiraStatus != "" || task.Priority != "" || task.IssueType != "" {
		desc.WriteString("**JIRA Info**:\n")
		if task.JiraStatus != "" {
			desc.WriteString(fmt.Sprintf("- Status: %s\n", task.JiraStatus))
		}
		if task.Priority != "" {
			desc.WriteString(fmt.Sprintf("- Priority: %s\n", task.Priority))
		}
		if task.IssueType != "" {
			desc.WriteString(fmt.Sprintf("- Type: %s\n", task.IssueType))
		}
		desc.WriteString("\n")
	}

	if task.NextSteps != "" {
		desc.WriteString("**Next Steps**:\n")
		desc.WriteString(task.NextSteps)
		desc.WriteString("\n\n")
	}

	if task.KeyFindings != "" {
		desc.WriteString("**Key Findings**:\n")
		desc.WriteString(task.KeyFindings)
		desc.WriteString("\n\n")
	}

	desc.WriteString("**Links**:\n")
	desc.WriteString(fmt.Sprintf("- [JIRA Ticket](https://alkiranet.atlassian.net/browse/%s)\n", task.ID))
	if task.PRLink != "" {
		desc.WriteString(fmt.Sprintf("- [Related PR](%s)\n", task.PRLink))
	}

	desc.WriteString(fmt.Sprintf("\n---\n*Last synced: %s*", time.Now().Format("2006-01-02 15:04")))

	return desc.String()
}

// AddLabelToCard adds a label to a Trello card
func (c *TrelloClient) AddLabelToCard(cardID, labelColor string) error {
	// Get card info to find board
	endpoint := fmt.Sprintf("/cards/%s", cardID)
	body, err := c.makeRequest(endpoint)
	if err != nil {
		return fmt.Errorf("failed to get card: %v", err)
	}

	var card struct {
		IDBoard string `json:"idBoard"`
	}
	if err := json.Unmarshal(body, &card); err != nil {
		return fmt.Errorf("failed to unmarshal card: %w", err)
	}

	// Get board labels
	labelsEndpoint := fmt.Sprintf("/boards/%s/labels", card.IDBoard)
	labelsBody, err := c.makeRequest(labelsEndpoint)
	if err != nil {
		return fmt.Errorf("failed to get board labels: %v", err)
	}

	var labels []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}

	if err := json.Unmarshal(labelsBody, &labels); err != nil {
		return fmt.Errorf("failed to unmarshal labels: %w", err)
	}

	// Find existing label or use first red label
	var labelID string
	for _, label := range labels {
		if label.Color == labelColor {
			labelID = label.ID
			break
		}
	}

	if labelID == "" {
		return fmt.Errorf("no %s label found on board", labelColor)
	}

	// Add label to card
	addLabelEndpoint := fmt.Sprintf("/cards/%s/idLabels", cardID)

	u, err := url.Parse(c.BaseURL + addLabelEndpoint)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("token", c.APIToken)
	q.Set("value", labelID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add label: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for debugging
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %s: %s", resp.Status, string(respBody))
	}

	return nil
}


// UpdateCardTitle updates the title of a Trello card
func (c *TrelloClient) UpdateCardTitle(cardID, title string) error {
	endpoint := fmt.Sprintf("/cards/%s", cardID)

	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("token", c.APIToken)
	q.Set("name", title)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("PUT", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update card title: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return nil
}

// mapListNameToStatus converts Trello list names to local status
func (c *TrelloClient) mapListNameToStatus(listName string) string {
	switch strings.ToLower(listName) {
	case "sprint", "backlog", "to do", "todo":
		return "🎯 PLANNED"
	case "doing", "in progress":
		return "🔄 IN PROGRESS"
	case "in review", "code review", "review":
		return "👀 IN REVIEW"
	case "done", "completed":
		return "✅ COMPLETED"
	default:
		return "🔄 " + strings.ToUpper(listName)
	}
}

// updateLocalTaskStatus updates the STATUS.md file with status from Trello
func (c *TrelloClient) updateLocalTaskStatus(tasksDir, taskID, newStatus string) error {
	statusFile := filepath.Join(tasksDir, taskID, "STATUS.md")

	// Read current STATUS.md
	content, err := os.ReadFile(statusFile)
	if err != nil {
		return fmt.Errorf("failed to read STATUS.md: %v", err)
	}

	statusContent := string(content)

	// Update the Current Status line
	statusRegex := regexp.MustCompile(`## Current Status:\s*(.+)`)
	if statusRegex.MatchString(statusContent) {
		statusContent = statusRegex.ReplaceAllString(statusContent, fmt.Sprintf("## Current Status: %s", newStatus))
	} else {
		// If no Current Status line exists, add one after the title
		titleRegex := regexp.MustCompile(`(# [^\n]+\n)`)
		if titleRegex.MatchString(statusContent) {
			statusContent = titleRegex.ReplaceAllString(statusContent, fmt.Sprintf("$1\n## Current Status: %s\n", newStatus))
		}
	}

	// Write back to file
	if err := os.WriteFile(statusFile, []byte(statusContent), 0644); err != nil {
		return fmt.Errorf("failed to write STATUS.md: %v", err)
	}

	return nil
}

// mapListNameToJiraStatus converts Trello list names to JIRA statuses (simplified)
func (c *TrelloClient) mapListNameToJiraStatus(listName string) string {
	switch strings.ToLower(listName) {
	case "sprint", "backlog", "to do", "todo":
		return "Open"
	case "doing", "in progress", "in review", "code review", "review":
		return "In Progress"
	case "done", "completed":
		return "" // Don't auto-update - manual closure required
	default:
		return "" // Don't update if we don't recognize the list
	}
}

// updateJiraStatus updates the JIRA ticket status using jira CLI with smart state matching
func (c *TrelloClient) updateJiraStatus(taskID, targetStatus string) error {
	if targetStatus == "" {
		return nil // Skip update for unrecognized statuses
	}

	// Try the generic status first, and if it fails, parse available transitions
	cmd := exec.Command("jira", "issue", "move", taskID, targetStatus)
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err == nil {
		fmt.Printf("    ✓ Updated JIRA %s to '%s'\n", taskID, targetStatus)
		return nil
	}

	// If generic status failed, parse available transitions from error
	outputStr := string(output)

	// Find the best matching state based on target status and available transitions
	var bestMatch string
	switch strings.ToLower(targetStatus) {
	case "open":
		// Look for states that suggest starting work
		bestMatch = c.findBestJiraState(outputStr, []string{
			"need requirements", "started development", "development started",
			"fix in progress", "in progress", "start", "begin",
		})
	case "in progress":
		// Look for states that suggest work in progress
		bestMatch = c.findBestJiraState(outputStr, []string{
			"fix in progress", "started development", "development started",
			"in progress", "progress", "working",
		})
	case "done":
		// Look for states that suggest completion
		bestMatch = c.findBestJiraState(outputStr, []string{
			"resolve issue", "close", "done", "complete", "finish",
			"resolved", "closed", "finished",
		})
	}

	if bestMatch == "" {
		fmt.Printf("    No suitable JIRA transition found for '%s'\n", targetStatus)
		return nil // Don't error, just skip
	}

	fmt.Printf("    Updating JIRA %s: '%s' -> '%s'\n", taskID, targetStatus, bestMatch)

	// Try the matched state
	cmd = exec.Command("jira", "issue", "move", taskID, bestMatch)
	cmd.Env = os.Environ()

	output, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update JIRA status: %v, output: %s", err, string(output))
	}

	fmt.Printf("    ✓ Updated JIRA %s to '%s'\n", taskID, bestMatch)
	return nil
}

// findBestJiraState finds the best matching JIRA state from available options
func (c *TrelloClient) findBestJiraState(issueOutput string, candidates []string) string {
	// Extract all available states from error message
	// Format: "Available states for issue AK-12345: 'State 1', 'State 2'"
	var availableStates []string

	// Look for the "Available states" line
	lines := strings.Split(issueOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Available states") {
			// Extract states from the line (they're in quotes)
			parts := strings.Split(line, ":")
			if len(parts) > 1 {
				statesStr := parts[1]
				// Find all text within single quotes
				re := regexp.MustCompile(`'([^']*)'`)
				matches := re.FindAllStringSubmatch(statesStr, -1)
				for _, match := range matches {
					if len(match) > 1 {
						availableStates = append(availableStates, match[1])
					}
				}
			}
			break
		}
	}

	// Find the best match from available states
	for _, candidate := range candidates {
		for _, available := range availableStates {
			if strings.Contains(strings.ToLower(available), strings.ToLower(candidate)) {
				return available
			}
		}
	}

	// If no partial match, return the first available state for some fallback
	if len(availableStates) > 0 {
		fmt.Printf("    Available states: %v\n", availableStates)
	}

	return ""
}

// DeleteCard deletes a Trello card
func (c *TrelloClient) DeleteCard(cardID string) error {
	endpoint := fmt.Sprintf("/cards/%s", cardID)

	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("token", c.APIToken)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("DELETE", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete card: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return nil
}

// DeleteAllCardsFromList removes all cards from a specific list
func (c *TrelloClient) DeleteAllCardsFromList(listID string) error {
	cards, err := c.GetCardsInList(listID)
	if err != nil {
		return fmt.Errorf("failed to get cards in list: %w", err)
	}

	fmt.Printf("Deleting %d cards from list...\n", len(cards))

	for _, card := range cards {
		fmt.Printf("Deleting card: %s\n", card.Name)
		if err := c.DeleteCard(card.ID); err != nil {
			return fmt.Errorf("failed to delete card %s: %w", card.Name, err)
		}
	}

	fmt.Printf("Successfully deleted %d cards!\n", len(cards))
	return nil
}

// AddCommentToCard adds a comment to a Trello card
func (c *TrelloClient) AddCommentToCard(cardID, text string) error {
	endpoint := fmt.Sprintf("/cards/%s/actions/comments", cardID)

	u, err := url.Parse(c.BaseURL + endpoint)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	q := u.Query()
	q.Set("key", c.APIKey)
	q.Set("token", c.APIToken)
	q.Set("text", text)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("POST", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %s", resp.Status)
	}

	return nil
}


// CreateDailySundownNotification creates a daily sundown notification card
func (c *TrelloClient) CreateDailySundownNotification(boardName string) error {
	fmt.Println("Creating daily sundown notification...")

	// Find the sundown notification list
	listID, err := c.FindListByName(boardName, "Sundown Notification (DO NOT ALTER)")
	if err != nil {
		return fmt.Errorf("failed to find Sundown Notification list: %w", err)
	}

	// Delete all existing cards from the list
	if err := c.DeleteAllCardsFromList(listID); err != nil {
		return fmt.Errorf("failed to clear existing cards: %w", err)
	}

	// Get todays sundown time
	sundownTime, err := GetTodaySundownTime()
	if err != nil {
		return fmt.Errorf("failed to get sundown time: %w", err)
	}

	// Create todays card
	today := time.Now()
	cardTitle := fmt.Sprintf("Sundown Notification - %s", today.Format("Monday, January 2, 2006"))

	// Create the card
	if err := c.CreateCard(listID, cardTitle, "", ""); err != nil {
		return fmt.Errorf("failed to create sundown card: %w", err)
	}

	// Find the card we just created to add a comment
	cards, err := c.GetCardsInList(listID)
	if err != nil {
		return fmt.Errorf("failed to get cards to find new card: %w", err)
	}

	if len(cards) == 0 {
		return fmt.Errorf("no cards found after creation")
	}

	// Use the first (and should be only) card
	newCard := cards[0]

	// Add comment with mention and sundown information
	comment := fmt.Sprintf("@nalani_farnsworth Sundown today (%s) is at %s 🌅",
		today.Format("Monday, January 2, 2006"),
		sundownTime)

	if err := c.AddCommentToCard(newCard.ID, comment); err != nil {
		return fmt.Errorf("failed to add comment to sundown card: %w", err)
	}

	fmt.Printf("✅ Created sundown notification card for %s\n", today.Format("January 2, 2006"))
	fmt.Printf("   Sundown time: %s\n", sundownTime)
	fmt.Printf("   Notified: @nalani_farnsworth\n")

	return nil
}
