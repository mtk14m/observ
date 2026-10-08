/** Returns next if it is a path inside obsrv, otherwise the home page. */
export function safeNext(next: unknown): string {
  return typeof next === 'string' && next.startsWith('/') && !next.startsWith('//') ? next : '/services'
}
