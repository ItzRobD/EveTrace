// Pre-rendered point markers for the activity chart.
//
// Chart.js `pointStyle` accepts an HTMLCanvasElement, so we draw each marker's
// PrimeIcons glyph onto a small offscreen canvas once and hand that to the chart.
// Canvas (rather than an <img> of an SVG) avoids async image-load races and lets
// us colour the glyph directly. The glyphs mirror the icons used in the events
// table so the chart and feed stay visually consistent.

import type { PointStyle } from 'chart.js';

export type ChartMarkerKind = 'kill' | 'cap' | 'critical';

interface MarkerSpec {
  glyph: string; // PrimeIcons private-use codepoint
  color: string;
}

// Codepoints from primeicons.css: pi-trophy, pi-bolt, pi-sparkles.
const SPECS: Record<ChartMarkerKind, MarkerSpec> = {
  kill: { glyph: '', color: '#f59e0b' }, // pi-trophy
  cap: { glyph: '', color: '#ef4444' }, // pi-bolt
  critical: { glyph: '', color: '#22c55e' }, // pi-sparkles
};

// Named-shape fallbacks used before the font loads or outside a browser, so the
// chart always renders something sensible.
const FALLBACK_SHAPE: Record<ChartMarkerKind, PointStyle> = {
  kill: 'star',
  cap: 'crossRot',
  critical: 'triangle',
};

const CANVAS_SIZE = 30; // px — larger than the data lines so markers stand out
const FONT_SIZE = 22;
const FONT_FAMILY = 'primeicons';
const HALO_WIDTH = 4; // contrasting outline so the glyph reads on top of a thick line
const HALO_COLOR = 'rgba(0, 0, 0, 0.75)';

const cache = new Map<ChartMarkerKind, HTMLCanvasElement>();

function draw(spec: MarkerSpec): HTMLCanvasElement {
  const canvas = document.createElement('canvas');
  canvas.width = CANVAS_SIZE;
  canvas.height = CANVAS_SIZE;
  const ctx = canvas.getContext('2d')!;
  ctx.font = `${FONT_SIZE}px '${FONT_FAMILY}'`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  const cx = CANVAS_SIZE / 2;
  const cy = CANVAS_SIZE / 2;
  // Dark halo first, then the coloured glyph on top, so it pops against any line.
  ctx.lineWidth = HALO_WIDTH;
  ctx.lineJoin = 'round';
  ctx.strokeStyle = HALO_COLOR;
  ctx.strokeText(spec.glyph, cx, cy);
  ctx.fillStyle = spec.color;
  ctx.fillText(spec.glyph, cx, cy);
  return canvas;
}

/**
 * Returns the point style for a marker: a pre-rendered canvas when available, or
 * a named Chart.js shape as a fallback.
 */
export function chartMarker(kind: ChartMarkerKind): PointStyle {
  if (typeof document === 'undefined') return FALLBACK_SHAPE[kind];
  let canvas = cache.get(kind);
  if (!canvas) {
    canvas = draw(SPECS[kind]);
    cache.set(kind, canvas);
  }
  return canvas;
}

let readyPromise: Promise<void> | null = null;

/**
 * Ensures the PrimeIcons font is loaded before markers are drawn, then clears any
 * canvases rendered too early (which would have drawn a blank/tofu glyph). Callers
 * should recompute their chart data once this resolves so the markers re-render.
 * Single-flight: safe to call repeatedly.
 */
export function prewarmChartMarkers(): Promise<void> {
  if (readyPromise) return readyPromise;
  const fonts = typeof document !== 'undefined' ? document.fonts : undefined;
  if (!fonts) {
    readyPromise = Promise.resolve();
    return readyPromise;
  }
  readyPromise = fonts
    .load(`${FONT_SIZE}px '${FONT_FAMILY}'`)
    .then(() => fonts.ready)
    .then(() => {
      cache.clear(); // re-render with the now-available font on next access
    })
    .catch(() => {
      /* fall back to whatever rendered; not fatal */
    });
  return readyPromise;
}
