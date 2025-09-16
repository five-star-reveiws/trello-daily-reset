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

## Moodle/Open LMS Sync

To enable daily sync from a Moodle/Open LMS site that shows a "Get the mobile app" footer (Mobile App web services enabled):

- Set environment variables:
  - `MOODLE_BASE_URL` — e.g. `https://ohsu.mrooms3.net`
  - `MOODLE_WSTOKEN` — a Mobile App service token (see below)
  - Optional: `MOODLE_SYNC_TO` — end date for included assignments (`YYYY-MM-DD`); defaults to 60 days ahead.

- Get a Mobile App token:
  - If allowed, visit: `https://<your-moodle>/login/token.php?service=moodle_mobile_app&username=<user>&password=<pass>`
  - If SSO is used, look in Profile → Mobile app or Security keys for a personal token.
  - If blocked, ask the site admin to enable Mobile services or issue a token.

- Run a one-off test:
  ```bash
  go run . --test-moodle
  ```

- Sync assignments to Trello Weekly list:
  ```bash
  # default: next 60 days
  go run . --sync-moodle

  # or specify a date (e.g., end of quarter)
  go run . --sync-moodle --moodle-to 2025-10-31
  ```

- Dry run (no Trello changes; just prints planned actions):
  ```bash
  go run . --sync-moodle-dry-run --moodle-to 2025-10-31
  ```

Cards are created/updated on the `Makai School` board → `Weekly` list. Descriptions include a marker like `Moodle Assignment ID: <id>` so re-runs update existing cards.

## Getting List ID

To find a list ID, you can:
1. Open a Trello board in your browser
2. Add `.json` to the end of the board URL
3. Look for the list you want in the JSON response and copy its `id` field
