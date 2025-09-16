package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
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

