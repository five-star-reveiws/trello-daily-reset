# Daily Sundown Notification Setup

This document explains how to set up the automated daily sundown notifications.

## Features

- **Hybrid Caching**: Fetches 30 days of sunset data at once, reducing API calls to ~12 per year
- **Local Cache**: Stores sunset data in `sunset_cache.json` for fast daily lookups
- **Auto-Refresh**: Cache automatically refreshes when expired
- **Location**: Configured for Orem, Utah coordinates (40.2969°N, 111.6946°W)
- **API**: Uses SunriseSunset.io for accurate sunset times

## GitHub Actions Setup

### 1. Repository Secrets

Add these secrets to your GitHub repository (Settings → Secrets and variables → Actions):

```
TRELLO_API_KEY=your_trello_api_key
TRELLO_API_TOKEN=your_trello_api_token
```

### 2. Workflow Schedule

The workflow runs daily at:
- **8:00 AM MDT** (Mountain Daylight Time, UTC-6)
- **8:00 AM MST** (Mountain Standard Time, UTC-7)

GitHub Actions automatically handles the timezone conversion.

### 3. Manual Execution

You can trigger the workflow manually:
1. Go to Actions tab in your GitHub repository
2. Select "Daily Sundown Notification"
3. Click "Run workflow"

## Local Usage

```bash
# Create today's sundown notification
./trello-client --sundown-notify "Your Board Name"

# First run will fetch and cache 30 days of data
# Subsequent runs will use cached data (fast)
```

## Cache Behavior

- **First Run**: Downloads 30 days of sunset data (~1 API call)
- **Daily Runs**: Uses cached data (0 API calls)
- **Cache Refresh**: Automatically fetches new data when cache expires
- **Location Change**: Cache invalidates if coordinates change

## Trello Board Requirements

1. **Board Name**: Update the workflow to use your board name
2. **List Name**: Must have a list named "Sundown Notification (DO NOT ALTER)"
3. **User**: The notification will mention @nalani_farnsworth

## Troubleshooting

### Cache Issues
```bash
# Delete cache to force refresh
rm sunset_cache.json
./trello-client --sundown-notify "Your Board"
```

### API Errors
- Check if SunriseSunset.io is accessible
- Verify Orem, UT coordinates are correct
- Check network connectivity

### Trello Errors
- Verify API credentials are set
- Check board and list names exist
- Ensure user @nalani_farnsworth exists on the board

## Time Zones

- **Cache**: Stores times in local Mountain Time (MST/MDT)
- **Display**: Shows times like "6:23 PM MST" or "7:15 PM MDT"
- **Workflow**: Automatically adjusts for daylight saving time