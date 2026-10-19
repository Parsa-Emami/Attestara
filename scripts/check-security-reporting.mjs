import { pathToFileURL } from 'node:url';

export const repository = 'Parsa-Emami/Attestara';
export const reportURL = `https://github.com/${repository}/security/advisories/new`;

// A source policy cannot enable a GitHub setting. Read the live public setting.
// https://docs.github.com/en/rest/repos/repos#check-if-private-vulnerability-reporting-is-enabled-for-a-repository
export async function checkSecurityReporting(fetcher = fetch) {
  const response = await fetcher(`https://api.github.com/repos/${repository}/private-vulnerability-reporting`, {
    method: 'GET', redirect: 'error', signal: AbortSignal.timeout(10000),
    headers: { Accept: 'application/vnd.github+json', 'X-GitHub-Api-Version': '2026-03-10', 'User-Agent': 'Attestara-security-reporting-check' },
  });
  if (response.status !== 200) throw new Error('GitHub reporting status is unavailable.');
  const body = await response.json();
  if (typeof body?.enabled !== 'boolean') throw new Error('GitHub reporting status is invalid.');
  return { repository, enabled: body.enabled, reportURL };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  try {
    const result = await checkSecurityReporting();
    console.log(JSON.stringify({ ...result, checkedAt: new Date().toISOString() }));
    process.exitCode = result.enabled ? 0 : 2;
  } catch {
    console.error('GitHub reporting status unavailable; do not claim that private reporting is enabled.');
    process.exitCode = 1;
  }
}
