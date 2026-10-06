# Design principles

obsrv's interface is inspired by the best observability tools: the density and the
"click on anything" exploration of Datadog, and the clean, calm look of Tsuga.
We take inspiration from them and copy nothing: obsrv has its own identity.

## Principles

1. **The data is the interface.** Use dense layouts, little chrome and no decorative illustrations.
   Every pixel either shows telemetry or helps you navigate it.
2. **Everything is clickable and leads somewhere.** A service name, a trace ID, an attribute value or a
   point on a chart opens a context menu: filter by it, exclude it, or go to the related logs, traces
   and metrics. Correlation is a click, not a query.
3. **Keep the context.** Details open in a **side panel** over the current view, as Datadog does, instead of
   navigating away. Closing it brings you back to where you were.
4. **The URL is the state.** Filters, time range and open panels are all in the URL, so every view can be shared as a link.
5. **One global time range.** It is always visible in the top bar and shared by every page. Dragging across a chart zooms the time range.
6. **Fast by default.** Results stream in, skeletons appear in under 100 ms, and charts use uPlot (canvas).
   No view should wait for the slowest query.
7. **Keyboard first.** `/` focuses search, `t` opens the time picker, `Esc` closes the panel, `j`/`k` move through lists.
8. **Works offline.** Fonts and assets are bundled. obsrv never calls a CDN, because it runs in air-gapped environments.

## Layout

```
┌────┬───────────────────────────────────────────────────────────┐
│    │  search / breadcrumb                 [ Past 1 hour ▾ ]    │  top bar
│ ◎  ├───────────┬───────────────────────────────────┬───────────┤
│ ≡  │  facets   │  chart (volume / latency)         │  side     │
│ ⌁  │  service  │───────────────────────────────────│  panel    │
│ ▤  │  level    │  results list / table             │  (detail) │
│ ⚠  │  env      │                                   │           │
└────┴───────────┴───────────────────────────────────┴───────────┘
 nav
```

The sidebar links to Services, Traces, Logs, Metrics, Dashboards and Alerts. It collapses to icons only.

## Visual language

- **Theme:** dark is the default, because people on call look at it at 3 a.m. Light is also fully supported.
  Every color is a CSS custom property in `web/src/styles/tokens.css`. Components never use raw color values.
- **Neutrals** carry 90% of the interface. A single accent (violet) marks interaction: focus, selection and primary actions.
- **Signal colors** are stable everywhere, so users learn them: logs, traces and metrics each have their own hue.
- **Status colors** are reserved for meaning (ok, warning, error) and never used for decoration. Log levels follow a fixed scale.
- **Type:** Inter for the UI and JetBrains Mono for data (IDs, attributes, log bodies, durations).
  Numbers use tabular figures so columns line up.
- **Density:** 13px base text and 28px rows in tables, with an optional "comfortable" mode.

## Accessibility

- Text contrast is at least WCAG AA in both themes.
- Status is never conveyed by color alone: it always comes with an icon or a label.
- Every interaction is reachable with the keyboard, with visible focus rings.
