# Trello School Automation

A Go-based automation system that syncs assignments from Canvas LMS and Moodle/Open LMS to Trello, with daily task management and weekly card creation.

## Features

- **Canvas LMS Integration**: Syncs assignments and tracks grades with REDO logic for scores < 90%
- **Moodle/Open LMS Integration**: Syncs assignments from MHA courses
- **Daily Task Reset**: Updates due dates for daily tasks
- **Weekly Card Creation**: Creates structured weekly cards for school subjects
- **GitHub Actions Automation**: Runs daily at 11 PM MDT via cloud workflows

## Setup

### 1. Trello API Credentials

Get your Trello API credentials:
- Go to https://trello.com/power-ups/admin to create a Power-Up and get your API key
- Navigate to the "API Key" tab and select "Generate a new API Key"
- Click the "Token" hyperlink next to your API key to generate a token
- Click "Allow" on the authorization screen

### 2. Environment Variables

Create a `.env` file with:
```bash
# Trello
TRELLO_API_KEY="your_api_key"
TRELLO_API_TOKEN="your_api_token"

# Canvas LMS
CANVAS_API_TOKEN="your_canvas_token"
CANVAS_BASE_URL="https://alpine.instructure.com"

# Moodle/Open LMS
MOODLE_WSTOKEN="your_moodle_token"
MOODLE_BASE_URL="https://ohsu.mrooms3.net"
```

### 3. Test Connections

```bash
# Test all integrations
go run . --test-canvas
go run . --test-moodle
go run . --cache  # View boards and lists
```

## Moodle/Open LMS Sync

To enable daily sync from a Moodle/Open LMS site that shows a "Get the mobile app" footer (Mobile App web services enabled):

- Set environment variables:
  - `MOODLE_BASE_URL` — e.g. `https://ohsu.mrooms3.net`
  - `MOODLE_WSTOKEN` — a Mobile App service token (see below)
  - Optional: `MOODLE_SYNC_TO` — end date for included assignments (`YYYY-MM-DD`); defaults to 3 months ahead.

- Get a Mobile App token:
  - **For OHSU/MHA**: Visit: `https://ohsu.mrooms3.net/login/token.php?service=moodle_mobile_app&username=29farnron&password=<password>`
  - **General**: Visit: `https://<your-moodle>/login/token.php?service=moodle_mobile_app&username=<user>&password=<pass>`
  - If SSO is used, look in Profile → Mobile app or Security keys for a personal token.
  - If blocked, ask the site admin to enable Mobile services or issue a token.
  - **Note**: You may need to log into the Moodle mobile app first to enable token generation.

- Run a one-off test:
  ```bash
  go run . --test-moodle
  ```

- Sync assignments to Trello Weekly list:
  ```bash
  # default: next 3 months
  go run . --sync-moodle

  # or specify a date (e.g., end of quarter)
  go run . --sync-moodle --moodle-to 2025-10-31
  ```

- Dry run (no Trello changes; just prints planned actions):
  ```bash
  go run . --sync-moodle-dry-run --moodle-to 2025-10-31
  ```

Cards are created/updated on the `Makai School` board → `Weekly` list. Descriptions include a marker like `Moodle Assignment ID: <id>` so re-runs update existing cards.

## Canvas LMS Sync

To sync assignments from Canvas (Alpine Instructure):

- Set environment variables:
  - `CANVAS_API_TOKEN` — Your Canvas API token
  - `CANVAS_BASE_URL` — e.g. `https://alpine.instructure.com`

- Get Canvas API token:
  - Log into Canvas → Account → Settings
  - Scroll to "Approved Integrations"
  - Click "+ New Access Token"
  - Enter purpose and generate token

- Test Canvas connection:
  ```bash
  go run . --test-canvas
  ```

- Sync Canvas assignments:
  ```bash
  go run . --sync-canvas
  ```

Canvas integration includes:
- Grade tracking with REDO logic for scores < 90%
- Automatic due date management
- Metadata storage in card descriptions
- Duplicate prevention via Canvas assignment IDs

## Daily Automation

The system runs automatically via GitHub Actions at 11 PM MDT daily:

1. **Cache Refresh** - Updates Trello board/list data
2. **Canvas Sync** - Pulls assignments and grades
3. **Moodle Sync** - Pulls MHA course assignments
4. **Daily Reset** - Updates due dates for daily tasks

Manual operations:
```bash
# Refresh cache
go run . --refresh

# Reset daily tasks manually
go run . --daily-reset

# Create weekly cards for next week
go run . --create-weekly
```

## Getting List ID

To find a list ID, you can:
1. Open a Trello board in your browser
2. Add `.json` to the end of the board URL
3. Look for the list you want in the JSON response and copy its `id` field
