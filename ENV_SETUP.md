# Environment Setup for Testing

## Quick Setup

1. **Copy the example file:**
   ```bash
   cp .env.testing.example .env.testing
   ```

2. **Edit `.env.testing` and fill in your values:**
   ```bash
   nano .env.testing
   # or
   vim .env.testing
   ```

3. **Fill in the required values:**
   ```
   SLACK_BOT_TOKEN=xoxb-your-actual-bot-token
   SLACK_APP_TOKEN=xapp-your-actual-app-token
   OPENAI_API_KEY=sk-your-actual-api-key
   LOG_LEVEL=debug
   MCP_FILESYSTEM_PATH=${HOME}
   ```

4. **Run the test:**
   ```bash
   ./test-multi-step.sh
   ```

## Environment Variables

### Required Variables

- **`SLACK_BOT_TOKEN`** - Your Slack bot token (starts with `xoxb-`)
  - Get it from: https://api.slack.com/apps → Your App → OAuth & Permissions
  
- **`SLACK_APP_TOKEN`** - Your Slack app-level token (starts with `xapp-`)
  - Get it from: https://api.slack.com/apps → Your App → Basic Information → App-Level Tokens
  
- **`OPENAI_API_KEY`** - Your OpenAI API key (starts with `sk-`)
  - Get it from: https://platform.openai.com/api-keys

### Optional Variables

- **`LOG_LEVEL`** - Logging level (default: `debug`)
  - Options: `debug`, `info`, `warn`, `error`
  
- **`MCP_FILESYSTEM_PATH`** - Path for filesystem MCP server (default: `$HOME`)
  - The directory the filesystem MCP server will have access to

## How It Works

The `test-multi-step.sh` script will:
1. Automatically load variables from `.env.testing` if it exists
2. Allow shell environment variables to override `.env.testing` values
3. Check that all required variables are set
4. Build the Docker image if needed
5. Run the application with debug logging

## Security Note

- `.env.testing` is in `.gitignore` and won't be committed
- `.env.testing.example` is a template you can commit
- Never commit actual tokens or API keys!

## Alternative: Use Shell Environment Variables

If you prefer not to use `.env.testing`, you can set variables directly:

```bash
export SLACK_BOT_TOKEN="xoxb-..."
export SLACK_APP_TOKEN="xapp-..."
export OPENAI_API_KEY="sk-..."
export LOG_LEVEL="debug"

./test-multi-step.sh
```

Shell environment variables will override `.env.testing` values.
