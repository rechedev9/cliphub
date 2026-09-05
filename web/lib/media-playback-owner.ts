type PlaybackOwner = {
  owner: object;
  pause: () => void;
};

let current: PlaybackOwner | null = null;

/** Claims the single audible media slot and pauses the previous owner. */
export function claimMediaPlayback(owner: object, pause: () => void): () => void {
  if (current?.owner !== owner) current?.pause();
  current = { owner, pause };

  return () => {
    if (current?.owner === owner) current = null;
  };
}
