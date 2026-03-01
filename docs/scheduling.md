# Scheduled Runs

Use scheduled jobs to run cache cleanup during low-usage hours.

## cron (Linux/macOS)

Weekly at 3:00 AM every Sunday:

```cron
0 3 * * 0 /usr/local/bin/dev-cache --scan ~/src --clean --yes >> ~/cache-cleaner.log 2>&1
0 3 * * 0 /usr/local/bin/git-cleaner --scan ~/src --clean >> ~/cache-cleaner.log 2>&1
0 3 * * 0 /usr/local/bin/mac-cache-cleaner --clean >> ~/cache-cleaner.log 2>&1
```

Notes:
- Update binary paths based on your install location (`which dev-cache`, `which git-cleaner`, `which mac-cache-cleaner`).
- Keep log redirection enabled for troubleshooting.

## launchd (macOS)

Create `~/Library/LaunchAgents/com.cache-cleaner.weekly.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.cache-cleaner.weekly</string>

  <key>ProgramArguments</key>
  <array>
    <string>/usr/local/bin/mac-cache-cleaner</string>
    <string>--clean</string>
  </array>

  <key>StartCalendarInterval</key>
  <dict>
    <key>Weekday</key>
    <integer>1</integer>
    <key>Hour</key>
    <integer>3</integer>
    <key>Minute</key>
    <integer>0</integer>
  </dict>

  <key>StandardOutPath</key>
  <string>/Users/YOUR_USER/cache-cleaner.log</string>
  <key>StandardErrorPath</key>
  <string>/Users/YOUR_USER/cache-cleaner.log</string>
</dict>
</plist>
```

Load and start:

```bash
launchctl load ~/Library/LaunchAgents/com.cache-cleaner.weekly.plist
launchctl start com.cache-cleaner.weekly
```

Unload:

```bash
launchctl unload ~/Library/LaunchAgents/com.cache-cleaner.weekly.plist
```

## Homebrew services

`brew services` is best for long-running daemons, not periodic CLI jobs. For periodic cleanup, prefer `cron` or `launchd`.
