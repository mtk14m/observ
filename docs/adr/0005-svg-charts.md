# 0005. Hand-written SVG charts

- Status: Accepted
- Date: 2026-10-07
- Amends: [0004](0004-vue-for-the-web-ui.md), which planned uPlot for time series.

## Context

The V0 charts show at most about 150 points per series and 8 series. They must follow our chart
rules: a crosshair that snaps to the nearest point, a tooltip with every series, legends, gaps
shown as breaks, theme tokens for dark and light mode, and accessible labels.

## Decision

Charts are small Vue components that render SVG directly (`web/src/components/charts`), with no
charting library. Series colors come from a fixed, colorblind-validated categorical palette
defined as CSS tokens.

## Consequences

- We control every pixel and every interaction, and the charts are testable with Vue Test Utils.
- We add no dependency, so the bundle stays small.
- SVG slows down at tens of thousands of points. If dense charts become necessary, we will
  revisit with a canvas renderer such as uPlot behind the same component API.
