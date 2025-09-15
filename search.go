package main

import (
	"fmt"
	"strings"
)

func normalizeString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func findBoardByName(boards []Board, boardName string) (*Board, error) {
	boardNameNorm := normalizeString(boardName)

	// Try exact match first
	for _, board := range boards {
		if normalizeString(board.Name) == boardNameNorm {
			return &board, nil
		}
	}

	// Try partial match
	for _, board := range boards {
		if strings.Contains(normalizeString(board.Name), boardNameNorm) {
			return &board, nil
		}
	}

	return nil, fmt.Errorf("board '%s' not found", boardName)
}

func findListByName(lists []List, boardID, listName string) (*List, error) {
	listNameNorm := normalizeString(listName)

	// Try exact match first
	for _, list := range lists {
		if list.BoardID == boardID && normalizeString(list.Name) == listNameNorm {
			return &list, nil
		}
	}

	// Try partial match
	for _, list := range lists {
		if list.BoardID == boardID && strings.Contains(normalizeString(list.Name), listNameNorm) {
			return &list, nil
		}
	}

	return nil, fmt.Errorf("list '%s' not found in board", listName)
}

func (c *TrelloClient) FindListByName(boardName, listName string) (string, error) {
	cache, err := c.LoadCache()
	if err != nil {
		return "", err
	}

	board, err := findBoardByName(cache.Boards, boardName)
	if err != nil {
		return "", err
	}

	list, err := findListByName(cache.Lists, board.ID, listName)
	if err != nil {
		return "", fmt.Errorf("%s in board '%s'", err.Error(), board.Name)
	}

	return list.ID, nil
}