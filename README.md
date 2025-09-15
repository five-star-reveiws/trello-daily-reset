# Trello Client

A simple Go client for the Trello API to get cards from a list.

## Setup

1. Get your Trello API credentials:
   - Go to https://trello.com/power-ups/admin to create a Power-Up and get your API key
   - Navigate to the "API Key" tab and select "Generate a new API Key"
   - Click the "Token" hyperlink next to your API key to generate a token
   - Click "Allow" on the authorization screen

2. Set environment variables:
   ```bash
   export TRELLO_API_KEY="your_api_key"
   export TRELLO_API_TOKEN="your_api_token"
   export TRELLO_LIST_ID="your_list_id"
   ```

3. Run the client:
   ```bash
   go run .
   ```

## Getting List ID

To find a list ID, you can:
1. Open a Trello board in your browser
2. Add `.json` to the end of the board URL
3. Look for the list you want in the JSON response and copy its `id` field