# Design principles

obsrv's interface is inspired by the best observability tools: the "click on anything"
exploration of Datadog, and the calm, flat, search-first layout of Tsuga (icon rail, filter
chips, tabs with counts, tree waterfalls). We take inspiration from them and copy no brand
assets: obsrv keeps its own logo and accent color.

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
6. **Fast by default.** Results load in the background while the previous ones stay visible, and charts are lightweight SVG (ADR 0005).
   No view should wait for the slowest query.
7. **Keyboard first.** `/` focuses search, `t` opens the time picker, `Esc` closes the panel, `j`/`k` move through lists.
8. **Works offline.** Fonts and assets are bundled. obsrv never calls a CDN, because it runs in air-gapped environments.

## Layout

```
┌────┬──────────────────────────────────────────────────────────────┐
│ ◎  │ ▤ Logs explorer                          [ ◷ Past 1 hour ⌄ ] │  page header
│    ├──────────────────────────────────────────────────────────────┤
│ ⬡  │ [⌕ Search for attribute:value or text …          ] [Search] │
│    │ [level is error ✕] [level ⌄] [service.name ⌄] [env ⌄] Clear  │  filter chips
│ ≡  ├──────────────────────────────────────────────────────────────┤
│ ▤  │ ▂▃▅▂▁▃▆▇▅▃  volume histogram                                 │
│ ⌁  ├──────────────────────────────────────────────────────────────┤
│    │ All logs (2.2K)                                              │  tabs with counts
│ ▦  │ Time        Level    Service    Message                      │
│ ⚠  │ 21:13:22    [Error]  payment    payment refused              │  side panel on click
│    │                                                              │
│ ☼  │                                                              │
└────┴──────────────────────────────────────────────────────────────┘
 icon rail
```

- **Icon rail.** A narrow rail of icons, grouped (services · signals · dashboards and alerts), with
  labels as tooltips. The active section is tinted with the accent. The theme toggle sits at the bottom.
- **Page header.** The section icon, a breadcrumb on detail pages (`Traces explorer / 5b4af9da`),
  and the time range on the right, only on pages that use it.
- **Search, then chips.** A large search field comes first. Active filters appear as removable
  chips (`level is error ✕`), and facet chips list the most frequent values with their counts.
- **Flat sections.** Areas are separated by hairlines rather than floating cards. Tabs carry
  counts (`All logs 2.2K`).

## Visual language

- **Theme:** dark is the default, because people on call look at it at 3 a.m. Light is also fully supported.
  Every color is a CSS custom property in `web/src/styles/tokens.css`. Components never use raw color values.
- **Neutrals** carry 90% of the interface. They are slightly warm grays, never blue-tinted. A
  single accent (violet, obsrv's own color) marks interaction: focus, selection, active
  navigation and primary actions. Changing the accent is a one-line token change.
- **Level pills.** Log levels and statuses use tinted pills (`Error` in soft red, `Warn` in amber)
  with a readable label, never color alone.
- **Signal colors** are stable everywhere, so users learn them: logs, traces and metrics each have their own hue.
- **Status colors** are reserved for meaning (ok, warning, error) and never used for decoration. Log levels follow a fixed scale.
- **Type:** Inter for the UI and JetBrains Mono for data (IDs, attributes, log bodies, durations).
  Numbers use tabular figures so columns line up.
- **Density:** 14px base text and 44px table rows. The interface is calm and roomy; the data is dense.

## Accessibility

- Text contrast is at least WCAG AA in both themes.
- Status is never conveyed by color alone: it always comes with an icon or a label.
- Every interaction is reachable with the keyboard, with visible focus rings.
