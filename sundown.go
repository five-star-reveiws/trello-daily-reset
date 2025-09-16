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

// SunriseSunsetIOResponse represents the response from SunriseSunset.io API
type SunriseSunsetIOResponse struct {
	Results []SunriseSunsetResult `json:"results"`
}

type SunriseSunsetResult struct {
	Date                     string `json:"date"`
	Sunrise                  string `json:"sunrise"`
	Sunset                   string `json:"sunset"`
	FirstLight               string `json:"first_light"`
	LastLight                string `json:"last_light"`
	Dawn                     string `json:"dawn"`
	Dusk                     string `json:"dusk"`
	SolarNoon                string `json:"solar_noon"`
	GoldenHour               string `json:"golden_hour"`
	DayLength                string `json:"day_length"`
	Timezone                 string `json:"timezone"`
	UTCOffset                int    `json:"utc_offset"`
}

// SunsetCache represents the local cache structure
type SunsetCache struct {
	Location    SunsetLocation          `json:"location"`
	CachedUntil time.Time              `json:"cached_until"`
	Data        map[string]string      `json:"data"` // date -> sunset time
}

type SunsetLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

const (
	sunsetCacheFile = "sunset_cache.json"
	oremLat         = 40.2969
	oremLng         = -111.6946
)

// GetSundownTime gets the sunset time for today using hybrid caching approach
func GetSundownTime(lat, lng float64) (string, error) {
	today := time.Now().Format("2006-01-02")

	// 1. Check local cache first
	if cachedTime := checkSunsetCache(today, lat, lng); cachedTime != "" {
		return cachedTime, nil
	}

	// 2. Cache miss - fetch next 30 days and cache
	fmt.Println("Cache miss - fetching sunset data for next 30 days...")
	return fetchAndCacheSunsetData(lat, lng, today)
}

// checkSunsetCache checks if we have valid cached data for today
func checkSunsetCache(dateStr string, lat, lng float64) string {
	data, err := os.ReadFile(sunsetCacheFile)
	if err != nil {
		return "" // No cache file
	}

	var cache SunsetCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return "" // Invalid cache file
	}

	// Check if cache is for same location
	if cache.Location.Latitude != lat || cache.Location.Longitude != lng {
		return "" // Different location
	}

	// Check if cache is still valid (not expired)
	if time.Now().After(cache.CachedUntil) {
		return "" // Cache expired
	}

	// Check if we have data for today
	if sunsetTime, exists := cache.Data[dateStr]; exists {
		return sunsetTime
	}

	return "" // No data for today
}

// fetchAndCacheSunsetData fetches 30 days of sunset data and caches it
func fetchAndCacheSunsetData(lat, lng float64, startDate string) (string, error) {
	// Parse start date
	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return "", fmt.Errorf("failed to parse start date: %w", err)
	}

	// Calculate end date (30 days from start)
	end := start.AddDate(0, 0, 29)

	// Build API URL for batch request
	apiURL := "https://api.sunrisesunset.io/json"
	u, err := url.Parse(apiURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse API URL: %w", err)
	}

	q := u.Query()
	q.Set("lat", fmt.Sprintf("%.6f", lat))
	q.Set("lng", fmt.Sprintf("%.6f", lng))
	q.Set("date_start", start.Format("2006-01-02"))
	q.Set("date_end", end.Format("2006-01-02"))
	q.Set("time_format", "24")
	u.RawQuery = q.Encode()

	// Make API request
	resp, err := http.Get(u.String())
	if err != nil {
		return "", fmt.Errorf("failed to make API request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResponse SunriseSunsetIOResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Process results and build cache
	cache := SunsetCache{
		Location: SunsetLocation{
			Latitude:  lat,
			Longitude: lng,
		},
		CachedUntil: end.AddDate(0, 0, 1), // Valid until day after end date
		Data:        make(map[string]string),
	}

	var todaySunset string

	for _, result := range apiResponse.Results {
		// Parse sunset time and convert to local time (API returns HH:MM:SS format)
		sunsetTime, err := time.Parse("15:04:05", result.Sunset)
		if err != nil {
			fmt.Printf("Warning: failed to parse sunset time '%s': %v\n", result.Sunset, err)
			continue // Skip invalid times
		}

		// Create full date-time for proper timezone conversion
		resultDate, err := time.Parse("2006-01-02", result.Date)
		if err != nil {
			continue
		}

		// Combine date and time
		fullSunset := time.Date(resultDate.Year(), resultDate.Month(), resultDate.Day(),
			sunsetTime.Hour(), sunsetTime.Minute(), 0, 0, time.UTC)

		// Convert to local timezone
		localSunset := fullSunset.Local()
		formattedTime := localSunset.Format("3:04 PM MST")

		// Store in cache
		cache.Data[result.Date] = formattedTime

		// Keep track of today's sunset
		if result.Date == startDate {
			todaySunset = formattedTime
		}
	}

	// Save cache to file
	cacheData, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal cache: %w", err)
	}

	if err := os.WriteFile(sunsetCacheFile, cacheData, 0644); err != nil {
		return "", fmt.Errorf("failed to write cache file: %w", err)
	}

	fmt.Printf("✅ Cached sunset data for 30 days (until %s)\n", end.Format("2006-01-02"))

	if todaySunset == "" {
		return "", fmt.Errorf("no sunset data found for today (%s)", startDate)
	}

	return todaySunset, nil
}

// GetTodaySundownTime gets sundown time for today using Orem, Utah coordinates
func GetTodaySundownTime() (string, error) {
	return GetSundownTime(oremLat, oremLng)
}