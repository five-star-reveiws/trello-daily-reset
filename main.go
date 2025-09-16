package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func main() {
	var (
		refresh      = flag.Bool("refresh", false, "Refresh cache from Trello API")
		showCache    = flag.Bool("cache", false, "Show cached boards and lists")
		board        = flag.String("board", "", "Board name to get cards from")
		list         = flag.String("list", "", "List name to get cards from")
		dailyReset   = flag.Bool("daily-reset", false, "Reset Makai's daily tasks with new due dates")
		createWeekly = flag.Bool("create-weekly", false, "Create weekly cards for next week")
		testCanvas   = flag.Bool("test-canvas", false, "Test Canvas API connection")
		syncCanvas   = flag.Bool("sync-canvas", false, "Sync Canvas assignments to Trello")
	)
	flag.Parse()

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	apiKey := os.Getenv("TRELLO_API_KEY")
	apiToken := os.Getenv("TRELLO_API_TOKEN")

	if apiKey == "" || apiToken == "" {
		log.Fatal("Please set TRELLO_API_KEY and TRELLO_API_TOKEN in .env file or environment variables")
	}

	client := NewTrelloClient(apiKey, apiToken)

	if *refresh {
		fmt.Println("Refreshing cache...")
		if err := client.CacheData(); err != nil {
			log.Fatalf("Failed to cache data: %v", err)
		}
		fmt.Println("Cache updated successfully!")
		return
	}

	if *dailyReset {
		fmt.Println("Resetting Makai's daily tasks...")
		if err := client.ResetDailyTasks("Makai School", "Daily"); err != nil {
			log.Fatalf("Failed to reset daily tasks: %v", err)
		}
		return
	}

	if *createWeekly {
		fmt.Println("Creating weekly cards for next week...")
		if err := client.CreateWeeklyCards(); err != nil {
			log.Fatalf("Failed to create weekly cards: %v", err)
		}
		return
	}

	if *testCanvas {
		canvasToken := os.Getenv("CANVAS_API_TOKEN")
		canvasURL := os.Getenv("CANVAS_BASE_URL")

		if canvasToken == "" || canvasURL == "" {
			log.Fatal("Please set CANVAS_API_TOKEN and CANVAS_BASE_URL in .env file or environment variables")
		}

		canvasClient := NewCanvasClient(canvasToken, canvasURL)
		fmt.Println("Testing Canvas API connection...")
		if err := canvasClient.TestConnection(); err != nil {
			log.Fatalf("Failed to connect to Canvas: %v", err)
		}
		return
	}

	if *syncCanvas {
		canvasToken := os.Getenv("CANVAS_API_TOKEN")
		canvasURL := os.Getenv("CANVAS_BASE_URL")

		if canvasToken == "" || canvasURL == "" {
			log.Fatal("Please set CANVAS_API_TOKEN and CANVAS_BASE_URL in .env file or environment variables")
		}

		canvasClient := NewCanvasClient(canvasToken, canvasURL)

		// Get Canvas user ID for grade lookups
		user, err := canvasClient.GetCurrentUser()
		if err != nil {
			log.Fatalf("Failed to get Canvas user: %v", err)
		}

		fmt.Printf("Syncing Canvas assignments for user: %s (ID: %d)\n", user.Name, user.ID)

		if err := client.SyncCanvasAssignments(canvasClient, user.ID); err != nil {
			log.Fatalf("Failed to sync Canvas assignments: %v", err)
		}
		return
	}

	if *showCache {
		cache, err := client.LoadCache()
		if err != nil {
			log.Fatalf("Failed to load cache: %v", err)
		}

		fmt.Printf("Cached boards and lists:\n")
		for _, board := range cache.Boards {
			fmt.Printf("- %s (ID: %s)\n", board.Name, board.ID)
			for _, list := range cache.Lists {
				if list.BoardID == board.ID {
					fmt.Printf("  └─ %s (ID: %s)\n", list.Name, list.ID)
				}
			}
			fmt.Println()
		}
		return
	}

	if *board != "" && *list != "" {
		listID, err := client.FindListByName(*board, *list)
		if err != nil {
			log.Fatalf("Failed to find list: %v", err)
		}

		cards, err := client.GetCardsInList(listID)
		if err != nil {
			log.Fatalf("Failed to get cards: %v", err)
		}

		fmt.Printf("Cards in '%s' -> '%s':\n", *board, *list)
		for _, card := range cards {
			fmt.Printf("- %s\n", card.Name)
			if card.Description != "" {
				fmt.Printf("  %s\n", card.Description)
			}
			fmt.Printf("  %s\n", card.URL)
			fmt.Println()
		}
		return
	}

	// Default: Get all boards (live data)
	boards, err := client.GetBoards()
	if err != nil {
		log.Fatalf("Failed to get boards: %v", err)
	}

	fmt.Printf("Found %d boards:\n", len(boards))
	for _, board := range boards {
		fmt.Printf("- %s (ID: %s)\n", board.Name, board.ID)

		lists, err := client.GetListsInBoard(board.ID)
		if err != nil {
			fmt.Printf("  Error getting lists: %v\n", err)
			continue
		}

		for _, list := range lists {
			fmt.Printf("  └─ %s (ID: %s)\n", list.Name, list.ID)
		}
		fmt.Println()
	}
}