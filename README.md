# btta

Arcade leaderboard display for Back to the Arcade.

## Usage

```
# Create an admin user
btta create-user <username>

# Start the server
btta serve --db btta.db --media-dir media
```

Other commands: `btta force-password-reset <username>`,
`btta regenerate-thumbnails [--force]`, `btta import --from <url>`.

The admin UI is available at `/admin/`.
