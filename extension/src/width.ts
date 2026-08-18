// Cell-width arithmetic, ANSI- and wide-char-aware.
//
// This is a deliberate re-implementation of the subset of
// @earendil-works/pi-tui's src/utils.ts that the statusline needs, NOT an
// import. pi loads this extension as bare .ts files copied out of the Nix
// store with no node_modules beside them, so a runtime dependency would fail
// at load.
//
// Re-implementing a width function is normally a mistake: pi's differential
// renderer measures with pi-tui's visibleWidth, so a one-cell disagreement
// corrupts the frame. So this is not an approximation of pi's algorithm, it is
// a port of it — same Intl.Segmenter, same Unicode property regexes, same
// East_Asian_Width table, same escape scanner, same truncation. width.test.ts
// pins it against the real pi-tui over a shared corpus plus 2000 fuzzed
// strings; if pi's ever changes, that test fails and this file is re-derived.
//
// The East_Asian_Width ranges below are vendored from `get-east-asian-width`
// (MIT, sindresorhus), which is what pi-tui itself calls. Only the fullwidth
// and wide tables are carried: pi calls eastAsianWidth() with the default
// ambiguousAsWide=false, so ambiguous/halfwidth/narrow/neutral are all one
// cell and their tables are dead weight.

const FULLWIDTH_MIN = 12288;
const FULLWIDTH_MAX = 65510;
// Flat [start, end, start, end, ...] inclusive pairs, sorted ascending.
const FULLWIDTH_RANGES: readonly number[] = [
  12288, 12288, 65281, 65376, 65504, 65510,
];

const WIDE_MIN = 4352;
const WIDE_MAX = 262141;
const WIDE_RANGES: readonly number[] = [
  4352, 4447, 8986, 8987, 9001, 9002, 9193, 9196, 9200, 9200, 9203, 9203, 9725, 9726,
  9748, 9749, 9776, 9783, 9800, 9811, 9855, 9855, 9866, 9871, 9875, 9875, 9889, 9889,
  9898, 9899, 9917, 9918, 9924, 9925, 9934, 9934, 9940, 9940, 9962, 9962, 9970, 9971,
  9973, 9973, 9978, 9978, 9981, 9981, 9989, 9989, 9994, 9995, 10024, 10024, 10060, 10060,
  10062, 10062, 10067, 10069, 10071, 10071, 10133, 10135, 10160, 10160, 10175, 10175,
  11035, 11036, 11088, 11088, 11093, 11093, 11904, 11929, 11931, 12019, 12032, 12245,
  12272, 12287, 12289, 12350, 12353, 12438, 12441, 12543, 12549, 12591, 12593, 12686,
  12688, 12773, 12783, 12830, 12832, 12871, 12880, 42124, 42128, 42182, 43360, 43388,
  44032, 55203, 63744, 64255, 65040, 65049, 65072, 65106, 65108, 65126, 65128, 65131,
  94176, 94180, 94192, 94198, 94208, 101589, 101631, 101662, 101760, 101874, 110576, 110579,
  110581, 110587, 110589, 110590, 110592, 110882, 110898, 110898, 110928, 110930,
  110933, 110933, 110948, 110951, 110960, 111355, 119552, 119638, 119648, 119670,
  126980, 126980, 127183, 127183, 127374, 127374, 127377, 127386, 127488, 127490,
  127504, 127547, 127552, 127560, 127568, 127569, 127584, 127589, 127744, 127776,
  127789, 127797, 127799, 127868, 127870, 127891, 127904, 127946, 127951, 127955,
  127968, 127984, 127988, 127988, 127992, 128062, 128064, 128064, 128066, 128252,
  128255, 128317, 128331, 128334, 128336, 128359, 128378, 128378, 128405, 128406,
  128420, 128420, 128507, 128591, 128640, 128709, 128716, 128716, 128720, 128722,
  128725, 128728, 128732, 128735, 128747, 128748, 128756, 128764, 128992, 129003,
  129008, 129008, 129292, 129338, 129340, 129349, 129351, 129535, 129648, 129660,
  129664, 129674, 129678, 129734, 129736, 129736, 129741, 129756, 129759, 129770,
  129775, 129784, 131072, 196605, 196608, 262141,
];

/** Binary search over a sorted flat array of inclusive [start, end] pairs. */
function isInRange(ranges: readonly number[], codePoint: number): boolean {
  let low = 0;
  let high = Math.floor(ranges.length / 2) - 1;
  while (low <= high) {
    const mid = Math.floor((low + high) / 2);
    const i = mid * 2;
    if (codePoint < ranges[i]!) {
      high = mid - 1;
    } else if (codePoint > ranges[i + 1]!) {
      low = mid + 1;
    } else {
      return true;
    }
  }
  return false;
}

/**
 * East_Asian_Width in cells, with ambiguous characters counted narrow — the
 * default get-east-asian-width applies and therefore what pi-tui gets.
 */
export function eastAsianWidth(codePoint: number): number {
  if (codePoint >= FULLWIDTH_MIN && codePoint <= FULLWIDTH_MAX && isInRange(FULLWIDTH_RANGES, codePoint)) {
    return 2;
  }
  if (codePoint >= WIDE_MIN && codePoint <= WIDE_MAX && isInRange(WIDE_RANGES, codePoint)) {
    return 2;
  }
  return 1;
}

// Shared segmenter, as pi-tui keeps one.
const graphemeSegmenter = new Intl.Segmenter(undefined, { granularity: "grapheme" });

const zeroWidthRegex = /^(?:\p{Default_Ignorable_Code_Point}|\p{Control}|\p{Mark}|\p{Surrogate})+$/v;
const leadingNonPrintingRegex = /^[\p{Default_Ignorable_Code_Point}\p{Control}\p{Format}\p{Mark}\p{Surrogate}]+/v;
const nonPrintingCharRegex = /^(?:\p{Default_Ignorable_Code_Point}|\p{Control}|\p{Format}|\p{Mark}|\p{Surrogate})$/v;
const markCharRegex = /^\p{Mark}$/v;
// Marks that terminals allocate cells for when attached to a base character.
// This includes Unicode spacing marks and non-spacing exceptions in legacy wcwidth tables.
const terminalSpacingMarkRegex =
  /^(?:[\p{Spacing_Mark}--[\u1734\u302E\u302F]]|[\u065F\u0F7F\u102B\u102C\u1031\u1033-\u1035\u1038\u103A-\u103E])+$/v;
const rgiEmojiRegex = /^\p{RGI_Emoji}$/v;

/**
 * Fast heuristic that avoids running the expensive RGI_Emoji regex on
 * everything. Deliberately broad, so future Unicode additions still reach the
 * real test.
 */
function couldBeEmoji(segment: string): boolean {
  const cp = segment.codePointAt(0)!;
  return (
    (cp >= 0x1f000 && cp <= 0x1fbff) ||
    (cp >= 0x2300 && cp <= 0x23ff) ||
    (cp >= 0x2600 && cp <= 0x27bf) ||
    (cp >= 0x2b50 && cp <= 0x2b55) ||
    segment.includes("\uFE0F") ||
    segment.length > 2
  );
}

function isPrintableAscii(str: string): boolean {
  for (let i = 0; i < str.length; i++) {
    const code = str.charCodeAt(i);
    if (code < 0x20 || code > 0x7e) {
      return false;
    }
  }
  return true;
}

/** Terminal cell width of one grapheme cluster. */
function graphemeWidth(segment: string): number {
  if (segment === "\t") {
    return 3;
  }
  // Some marks occupy cells even without a base character.
  if (terminalSpacingMarkRegex.test(segment)) {
    return [...segment].length;
  }
  if (zeroWidthRegex.test(segment)) {
    return 0;
  }
  if (couldBeEmoji(segment) && rgiEmojiRegex.test(segment)) {
    return 2;
  }
  const base = segment.replace(leadingNonPrintingRegex, "");
  const cp = base.codePointAt(0);
  if (cp === undefined) {
    return 0;
  }
  // Regional indicators render full-width in terminals even in isolation.
  if (cp >= 0x1f1e6 && cp <= 0x1f1ff) {
    return 2;
  }
  let width = eastAsianWidth(cp);
  // Intl.Segmenter can group several cell-occupying code points into one
  // grapheme; count the ones terminals allocate cells for.
  let followsMark = false;
  const chars = [...base];
  for (const char of chars.slice(1)) {
    if (terminalSpacingMarkRegex.test(char)) {
      width += 1;
      followsMark = false;
    } else if (markCharRegex.test(char)) {
      followsMark = true;
    } else if (!nonPrintingCharRegex.test(char)) {
      const c = char.codePointAt(0)!;
      if (followsMark || (c >= 0xff00 && c <= 0xffef)) {
        width += eastAsianWidth(c);
      } else if (c === 0x0e33 || c === 0x0eb3) {
        width += 1;
      }
      followsMark = false;
    }
  }
  return width;
}

/**
 * Extract the escape sequence at `pos`, or null. CSI runs to one of m/G/K/H/J;
 * OSC and APC run to BEL or ST. This is the scanner pi-tui uses, and it is why
 * an OSC 8 hyperlink's URL costs no cells.
 */
export function extractAnsiCode(str: string, pos: number): { code: string; length: number } | null {
  if (pos >= str.length || str[pos] !== "\x1b") return null;
  const next = str[pos + 1];
  if (next === "[") {
    let j = pos + 2;
    while (j < str.length && !/[mGKHJ]/.test(str[j]!)) j++;
    if (j < str.length) return { code: str.substring(pos, j + 1), length: j + 1 - pos };
    return null;
  }
  if (next === "]" || next === "_") {
    let j = pos + 2;
    while (j < str.length) {
      if (str[j] === "\x07") return { code: str.substring(pos, j + 1), length: j + 1 - pos };
      if (str[j] === "\x1b" && str[j + 1] === "\\") {
        return { code: str.substring(pos, j + 2), length: j + 2 - pos };
      }
      j++;
    }
    return null;
  }
  return null;
}

/** Remove ANSI/OSC/APC control sequences, preserving visible text. */
export function stripAnsi(str: string): string {
  if (!str.includes("\x1b")) return str;
  let out = "";
  let i = 0;
  while (i < str.length) {
    const ansi = extractAnsiCode(str, i);
    if (ansi) {
      i += ansi.length;
      continue;
    }
    out += str[i];
    i++;
  }
  return out;
}

const WIDTH_CACHE_SIZE = 512;
const widthCache = new Map<string, number>();

/** On-screen cell width, ignoring escape sequences. Tabs count as 3, as pi does. */
export function visibleWidth(str: string): number {
  if (str.length === 0) return 0;
  if (isPrintableAscii(str)) return str.length;
  const cached = widthCache.get(str);
  if (cached !== undefined) return cached;

  let clean = str;
  if (clean.includes("\t")) clean = clean.replace(/\t/g, "   ");
  if (clean.includes("\x1b")) clean = stripAnsi(clean);

  let width = 0;
  for (const { segment } of graphemeSegmenter.segment(clean)) {
    width += graphemeWidth(segment);
  }

  if (widthCache.size >= WIDTH_CACHE_SIZE) {
    const firstKey = widthCache.keys().next().value;
    if (firstKey !== undefined) widthCache.delete(firstKey);
  }
  widthCache.set(str, width);
  return width;
}

type Osc8Terminator = "\x07" | "\x1b\\";

/**
 * The OSC 8 close sequence that a truncated prefix still needs, or "". Cutting
 * mid-hyperlink and not closing it leaves the rest of the terminal line linked.
 */
function getActiveOsc8Close(prefix: string): string {
  if (!prefix.includes("\x1b]8;")) return "";
  let terminator: Osc8Terminator | undefined;
  let i = 0;
  while (i < prefix.length) {
    const ansi = extractAnsiCode(prefix, i);
    if (ansi) {
      if (ansi.code.startsWith("\x1b]8;")) {
        const term: Osc8Terminator = ansi.code.endsWith("\x07") ? "\x07" : "\x1b\\";
        const bodyText = ansi.code.slice(4, term === "\x07" ? -1 : -2);
        const sep = bodyText.indexOf(";");
        if (sep !== -1) terminator = bodyText.slice(sep + 1) ? term : undefined;
      }
      i += ansi.length;
    } else {
      i++;
    }
  }
  return terminator ? `\x1b]8;;${terminator}` : "";
}

function truncateFragmentToWidth(text: string, maxWidth: number): { text: string; width: number } {
  if (maxWidth <= 0 || text.length === 0) return { text: "", width: 0 };
  if (isPrintableAscii(text)) {
    const clipped = text.slice(0, maxWidth);
    return { text: clipped, width: clipped.length };
  }
  let result = "";
  let width = 0;
  let i = 0;
  let pendingAnsi = "";
  while (i < text.length) {
    const ansi = extractAnsiCode(text, i);
    if (ansi) {
      pendingAnsi += ansi.code;
      i += ansi.length;
      continue;
    }
    if (text[i] === "\t") {
      if (width + 3 > maxWidth) break;
      if (pendingAnsi) {
        result += pendingAnsi;
        pendingAnsi = "";
      }
      result += "\t";
      width += 3;
      i++;
      continue;
    }
    let end = i;
    while (end < text.length && text[end] !== "\t" && !extractAnsiCode(text, end)) end++;
    for (const { segment } of graphemeSegmenter.segment(text.slice(i, end))) {
      const w = graphemeWidth(segment);
      if (width + w > maxWidth) return { text: result, width };
      if (pendingAnsi) {
        result += pendingAnsi;
        pendingAnsi = "";
      }
      result += segment;
      width += w;
    }
    i = end;
  }
  return { text: result, width };
}

function finalizeTruncatedResult(
  prefix: string,
  prefixWidth: number,
  ellipsis: string,
  ellipsisWidth: number,
  maxWidth: number,
  pad: boolean,
): string {
  const reset = "\x1b[0m";
  const hyperlinkClose = getActiveOsc8Close(prefix);
  const visible = prefixWidth + ellipsisWidth;
  const result =
    ellipsis.length > 0
      ? `${prefix}${hyperlinkClose}${reset}${ellipsis}${reset}`
      : `${prefix}${hyperlinkClose}${reset}`;
  return pad ? result + " ".repeat(Math.max(0, maxWidth - visible)) : result;
}

/**
 * Truncate to `max` cells, appending an ellipsis when anything was dropped.
 * A port of pi-tui's truncateToWidth (default ellipsis "..."), so a line we
 * already fitted is a fixed point of pi's own truncator.
 */
export function truncateEnd(text: string, max: number, ellipsis = "..."): string {
  if (max <= 0) return "";
  if (text.length === 0) return "";

  const ellipsisWidth = visibleWidth(ellipsis);
  if (ellipsisWidth >= max) {
    const textWidth = visibleWidth(text);
    if (textWidth <= max) return text;
    const clipped = truncateFragmentToWidth(ellipsis, max);
    if (clipped.width === 0) return "";
    return finalizeTruncatedResult("", 0, clipped.text, clipped.width, max, false);
  }

  if (isPrintableAscii(text)) {
    if (text.length <= max) return text;
    const targetWidth = max - ellipsisWidth;
    return finalizeTruncatedResult(text.slice(0, targetWidth), targetWidth, ellipsis, ellipsisWidth, max, false);
  }

  const targetWidth = max - ellipsisWidth;
  let result = "";
  let pendingAnsi = "";
  let visibleSoFar = 0;
  let keptWidth = 0;
  let keepContiguousPrefix = true;
  let overflowed = false;
  let exhaustedInput = false;
  const hasAnsi = text.includes("\x1b");
  const hasTabs = text.includes("\t");

  if (!hasAnsi && !hasTabs) {
    for (const { segment } of graphemeSegmenter.segment(text)) {
      const width = graphemeWidth(segment);
      if (keepContiguousPrefix && keptWidth + width <= targetWidth) {
        result += segment;
        keptWidth += width;
      } else {
        keepContiguousPrefix = false;
      }
      visibleSoFar += width;
      if (visibleSoFar > max) {
        overflowed = true;
        break;
      }
    }
    exhaustedInput = !overflowed;
  } else {
    let i = 0;
    while (i < text.length) {
      const ansi = extractAnsiCode(text, i);
      if (ansi) {
        pendingAnsi += ansi.code;
        i += ansi.length;
        continue;
      }
      if (text[i] === "\t") {
        if (keepContiguousPrefix && keptWidth + 3 <= targetWidth) {
          if (pendingAnsi) {
            result += pendingAnsi;
            pendingAnsi = "";
          }
          result += "\t";
          keptWidth += 3;
        } else {
          keepContiguousPrefix = false;
          pendingAnsi = "";
        }
        visibleSoFar += 3;
        if (visibleSoFar > max) {
          overflowed = true;
          break;
        }
        i++;
        continue;
      }
      let end = i;
      while (end < text.length && text[end] !== "\t" && !extractAnsiCode(text, end)) end++;
      for (const { segment } of graphemeSegmenter.segment(text.slice(i, end))) {
        const width = graphemeWidth(segment);
        if (keepContiguousPrefix && keptWidth + width <= targetWidth) {
          if (pendingAnsi) {
            result += pendingAnsi;
            pendingAnsi = "";
          }
          result += segment;
          keptWidth += width;
        } else {
          keepContiguousPrefix = false;
          pendingAnsi = "";
        }
        visibleSoFar += width;
        if (visibleSoFar > max) {
          overflowed = true;
          break;
        }
      }
      if (overflowed) break;
      i = end;
    }
    exhaustedInput = i >= text.length;
  }

  if (!overflowed && exhaustedInput) return text;
  return finalizeTruncatedResult(result, keptWidth, ellipsis, ellipsisWidth, max, false);
}

/**
 * Truncate from the middle, keeping the head and the tail. A port of Go's
 * render.TruncateMiddle: long bash commands and deep paths are unreadable when
 * only the head survives. Assumes plain text, as the Go one does — callers
 * truncate before colouring.
 */
export function truncateMiddle(s: string, max: number): string {
  if (max < 1) return "";
  if (visibleWidth(s) <= max) return s;
  if (max === 1) return "…";
  const avail = max - 1; // one cell for the ellipsis
  const headW = Math.floor((avail + 1) / 2);
  const tailW = avail - headW;

  const chars = Array.from(s);
  let head = "";
  let used = 0;
  let hi = 0;
  for (; hi < chars.length; hi++) {
    const w = graphemeWidth(chars[hi]!);
    if (used + w > headW) break;
    head += chars[hi];
    used += w;
  }
  used = 0;
  let ti = chars.length;
  while (ti > hi) {
    const w = graphemeWidth(chars[ti - 1]!);
    if (used + w > tailW) break;
    used += w;
    ti--;
  }
  return head + "…" + chars.slice(ti).join("");
}

/** Right-pad to exactly `width` cells. Never shrinks. */
export function padTo(s: string, width: number): string {
  const w = visibleWidth(s);
  return w >= width ? s : s + " ".repeat(width - w);
}
