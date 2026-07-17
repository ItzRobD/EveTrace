// Pre-rendered point markers for the activity chart.
//
// Chart.js `pointStyle` accepts an HTMLCanvasElement, so we draw each marker once
// onto a small offscreen canvas and hand that to the chart. Canvas (rather than an
// <img> of an SVG) avoids async image-load races and lets us colour markers
// directly. Glyph markers use PrimeIcons codepoints so the chart and the events
// feed stay visually consistent; the kill marker is a drawn explosion burst
// (no icon font has one) so it renders identically on every OS.

import type { PointStyle } from 'chart.js';

export type ChartMarkerKind = 'kill' | 'cap' | 'critical';

type MarkerSpec =
  | { render: 'glyph'; glyph: string; color: string }
  | { render: 'starburst'; color: string };

// Glyph codepoints from primeicons.css.
const SPECS: Record<ChartMarkerKind, MarkerSpec> = {
  kill: { render: 'starburst', color: '#ff5a2c' }, // explosion burst — ship destroyed
  cap: { render: 'glyph', glyph: '', color: '#ef4444' }, // pi-bolt
  critical: { render: 'glyph', glyph: '', color: '#22c55e' }, // pi-sparkles
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
const HALO_WIDTH = 4; // contrasting outline so the marker reads on top of a thick line
const HALO_COLOR = 'rgba(0, 0, 0, 0.75)';

const cache = new Map<ChartMarkerKind, HTMLCanvasElement>();

// drawStarburst renders a spiky explosion burst centred at (cx, cy). Drawn as a
// path rather than a glyph because no icon font ships an explosion, and a path
// looks identical across operating systems.
function drawStarburst(ctx: CanvasRenderingContext2D, cx: number, cy: number, color: string): void {
  const spikes = 10;
  const rOuter = 11;
  const rInner = 4.5;
  ctx.beginPath();
  for (let i = 0; i < spikes * 2; i++) {
    const r = i % 2 ? rInner : rOuter;
    const a = (Math.PI / spikes) * i - Math.PI / 2;
    const px = cx + Math.cos(a) * r;
    const py = cy + Math.sin(a) * r;
    if (i === 0) ctx.moveTo(px, py);
    else ctx.lineTo(px, py);
  }
  ctx.closePath();
  // Dark halo first, then the coloured fill, so it pops against any line.
  ctx.lineWidth = HALO_WIDTH;
  ctx.lineJoin = 'round';
  ctx.strokeStyle = HALO_COLOR;
  ctx.stroke();
  ctx.fillStyle = color;
  ctx.fill();
}

function draw(spec: MarkerSpec): HTMLCanvasElement {
  const canvas = document.createElement('canvas');
  canvas.width = CANVAS_SIZE;
  canvas.height = CANVAS_SIZE;
  const ctx = canvas.getContext('2d')!;
  const cx = CANVAS_SIZE / 2;
  const cy = CANVAS_SIZE / 2;

  if (spec.render === 'starburst') {
    drawStarburst(ctx, cx, cy, spec.color);
    return canvas;
  }

  ctx.font = `${FONT_SIZE}px '${FONT_FAMILY}'`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
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
 * Ensures the PrimeIcons font is loaded before glyph markers are drawn, then
 * clears any canvases rendered too early (which would have drawn a blank/tofu
 * glyph). Callers should recompute their chart data once this resolves so the
 * markers re-render. Single-flight: safe to call repeatedly.
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
