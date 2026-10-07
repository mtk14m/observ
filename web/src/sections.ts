/** The top-level sections of the UI, in navigation order. */
export interface Section {
  path: string
  name: string
  icon: IconName
  /** Empty-state hint shown until the section has data. */
  hint: string
}

export type IconName = 'services' | 'traces' | 'logs' | 'metrics' | 'dashboards' | 'alerts'

export const SECTIONS: readonly Section[] = [
  { path: '/services', name: 'Services', icon: 'services', hint: 'Services appear as soon as obsrv receives spans.' },
  { path: '/traces', name: 'Traces', icon: 'traces', hint: 'Send spans over OTLP to start exploring traces.' },
  { path: '/logs', name: 'Logs', icon: 'logs', hint: 'Send log records over OTLP to start searching logs.' },
  { path: '/metrics', name: 'Metrics', icon: 'metrics', hint: 'Send metrics over OTLP to start building charts.' },
  { path: '/dashboards', name: 'Dashboards', icon: 'dashboards', hint: 'Dashboards are coming soon. Explore metrics in the meantime.' },
  { path: '/alerts', name: 'Alerts', icon: 'alerts', hint: 'Get notified when errors spike or latency degrades.' },
]
