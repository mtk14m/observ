/** The top-level sections of the UI, in navigation order. */
export interface Section {
  path: string
  name: string
  icon: IconName
  /** Empty-state hint shown until the section has data. */
  hint: string
  /** Sections of the same group sit together in the navigation rail. */
  group: number
}

export type IconName = 'home' | 'issues' | 'services' | 'traces' | 'logs' | 'metrics' | 'dashboards' | 'alerts'

export const SECTIONS: readonly Section[] = [
  { path: '/', name: 'Home', icon: 'home', group: 0, hint: 'What needs attention right now.' },
  { path: '/services', name: 'Services', icon: 'services', group: 0, hint: 'Services appear as soon as obsrv receives spans.' },
  { path: '/issues', name: 'Issues', icon: 'issues', group: 0, hint: 'Errors grouped into issues.' },
  { path: '/traces', name: 'Traces', icon: 'traces', group: 1, hint: 'Send spans over OTLP to start exploring traces.' },
  { path: '/logs', name: 'Logs', icon: 'logs', group: 1, hint: 'Send log records over OTLP to start searching logs.' },
  { path: '/metrics', name: 'Metrics', icon: 'metrics', group: 1, hint: 'Send metrics over OTLP to start building charts.' },
  { path: '/dashboards', name: 'Dashboards', icon: 'dashboards', group: 2, hint: 'Dashboards are coming soon. Explore metrics in the meantime.' },
  { path: '/alerts', name: 'Alerts', icon: 'alerts', group: 2, hint: 'Get notified when errors spike or latency degrades.' },
]
