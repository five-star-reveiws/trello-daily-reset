package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Week struct {
	Number    int    `json:"number"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

type Quarter struct {
	Name      string   `json:"name"`
	StartDate string   `json:"startDate"`
	EndDate   string   `json:"endDate"`
	Subjects  []string `json:"subjects"`
	Weeks     []Week   `json:"weeks"`
}

type SubjectsConfig struct {
	Quarters []Quarter `json:"quarters"`
}

func LoadSubjectsConfig() (*SubjectsConfig, error) {
	data, err := os.ReadFile("subjects.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read subjects.json: %w", err)
	}

	var config SubjectsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal subjects config: %w", err)
	}

	return &config, nil
}

func (c *SubjectsConfig) GetCurrentQuarter() (*Quarter, error) {
	now := time.Now()

	for _, quarter := range c.Quarters {
		startDate, err := time.Parse("2006-01-02", quarter.StartDate)
		if err != nil {
			continue
		}
		endDate, err := time.Parse("2006-01-02", quarter.EndDate)
		if err != nil {
			continue
		}

		if now.After(startDate) && now.Before(endDate.AddDate(0, 0, 1)) {
			return &quarter, nil
		}
	}

	return nil, fmt.Errorf("no current quarter found for date %s", now.Format("2006-01-02"))
}

func (q *Quarter) GetCurrentWeek() (*Week, error) {
	now := time.Now()

	for _, week := range q.Weeks {
		startDate, err := time.Parse("2006-01-02", week.StartDate)
		if err != nil {
			continue
		}
		endDate, err := time.Parse("2006-01-02", week.EndDate)
		if err != nil {
			continue
		}

		if now.After(startDate.AddDate(0, 0, -1)) && now.Before(endDate.AddDate(0, 0, 1)) {
			return &week, nil
		}
	}

	return nil, fmt.Errorf("no current week found for date %s", now.Format("2006-01-02"))
}

func (q *Quarter) GetNextWeek(currentWeek *Week) (*Week, error) {
	for i, week := range q.Weeks {
		if week.Number == currentWeek.Number && i+1 < len(q.Weeks) {
			return &q.Weeks[i+1], nil
		}
	}

	return nil, fmt.Errorf("no next week found after week %d", currentWeek.Number)
}

func (q *Quarter) FormatWeekRange(week *Week) string {
	startDate, _ := time.Parse("2006-01-02", week.StartDate)
	endDate, _ := time.Parse("2006-01-02", week.EndDate)

	return fmt.Sprintf("%s %d–%d",
		startDate.Format("January"),
		startDate.Day(),
		endDate.Day())
}