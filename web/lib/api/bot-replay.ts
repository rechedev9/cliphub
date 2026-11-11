/** The body fields and response keys the library proxy lets through for the bot replay route. */
export const BOT_REPLAY_FIELDS = ['side', 'replace_steamid'] as const;
export const BOT_REPLAY_KEYS = [
  'launched', 'already_running', 'cfg_path', 'exec_command', 'map', 'round', 'side', 'bot_side',
  'keys', 'bots', 'starts', 'current', 'moments', 'lineups', 'limits',
] as const;
