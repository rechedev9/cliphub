const LATEST_RELEASE_API = 'https://api.github.com/repos/rechedev9/cliphub/releases/latest';
const RELEASES_PAGE = 'https://github.com/rechedev9/cliphub/releases/latest';
const INSTALLER_NAME = /^ClipHub\.Studio\.Setup\.\d+\.\d+\.\d+\.exe$/;

export async function GET(): Promise<Response> {
  let response: Response;
  try {
    response = await fetch(LATEST_RELEASE_API, {
      headers: { Accept: 'application/vnd.github+json', 'User-Agent': 'ClipHub-Web' },
      next: { revalidate: 300 },
    });
  } catch {
    return Response.redirect(RELEASES_PAGE, 307);
  }
  if (!response.ok) return Response.redirect(RELEASES_PAGE, 307);
  const body = await response.json() as unknown;
  if (typeof body !== 'object' || body === null || !Array.isArray((body as Record<string, unknown>).assets)) {
    return Response.redirect(RELEASES_PAGE, 307);
  }
  for (const asset of (body as { assets: unknown[] }).assets) {
    if (typeof asset !== 'object' || asset === null) continue;
    const candidate = asset as Record<string, unknown>;
    if (
      typeof candidate.name === 'string'
      && INSTALLER_NAME.test(candidate.name)
      && typeof candidate.browser_download_url === 'string'
      && validGitHubDownload(candidate.browser_download_url)
    ) {
      return Response.redirect(candidate.browser_download_url, 307);
    }
  }
  return Response.redirect(RELEASES_PAGE, 307);
}

function validGitHubDownload(value: string): boolean {
  try {
    const parsed = new URL(value);
    return parsed.protocol === 'https:'
      && parsed.hostname === 'github.com'
      && parsed.pathname.startsWith('/rechedev9/cliphub/releases/download/');
  } catch {
    return false;
  }
}
