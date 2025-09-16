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
