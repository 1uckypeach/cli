#!/usr/bin/env -S npx tsx
// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT
//
// CLI: measure single-line text width via CanvasKit, reusing ee/slide's core
// measure function so widths match the on-line renderer.
//
// Input is a Slide XML string (or an explicit --text). XML paragraph/style
// extraction mirrors xml_text_overlap_lint.py (extract_text_paragraphs /
// extract_max_span_font_size / style attribute names).
//
// Usage:
//   tsx cli.ts --font "Noto Sans SC=/abs/NotoSansSC.ttf" [--font ...] --file slide.xml
//   tsx cli.ts --font "Arial=/abs/Arial.ttf" --text "Hello" --font-size 16 [--bold] [--italic] [--letter-spacing 0]
//   cat slide.xml | tsx cli.ts --font "Arial=/abs/Arial.ttf"
//
// Output: JSON to stdout. Progress/warnings to stderr.

import { readFile } from 'node:fs/promises';

import { StandaloneMeasureRuntime, type FontSpec } from './runtime.ts';
import { measureSingleLineText, type SingleLineStyle } from './measure.ts';

interface CliArgs {
  fonts: FontSpec[];
  file?: string;
  xml?: string;
  text?: string;
  fontFamily: string;
  fontSize: string;
  letterSpacing: string;
  bold: boolean;
  italic: boolean;
}

interface Segment {
  shapeId: string | null;
  text: string;
  style: SingleLineStyle;
}

function parseArgs(argv: string[]): CliArgs {
  const args: CliArgs = {
    fonts: [],
    fontFamily: '',
    fontSize: '16px',
    letterSpacing: '0px',
    bold: false,
    italic: false,
  };
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    const next = () => {
      const value = argv[++i];
      if (value === undefined) {
        throw new Error(`Missing value for ${arg}`);
      }
      return value;
    };
    switch (arg) {
      case '--font': {
        // "Family Name=/abs/path.ttf" — split on the FIRST '=' so family names
        // may not contain '=', which is fine for font-family strings.
        const spec = next();
        const eq = spec.indexOf('=');
        if (eq < 0) {
          throw new Error(`--font expects "family=path", got: ${spec}`);
        }
        args.fonts.push({ family: spec.slice(0, eq).trim(), path: spec.slice(eq + 1).trim() });
        break;
      }
      case '--file':
        args.file = next();
        break;
      case '--xml':
        args.xml = next();
        break;
      case '--text':
        args.text = next();
        break;
      case '--font-family':
        args.fontFamily = next();
        break;
      case '--font-size':
        args.fontSize = next();
        break;
      case '--letter-spacing':
        args.letterSpacing = next();
        break;
      case '--bold':
        args.bold = true;
        break;
      case '--italic':
        args.italic = true;
        break;
      default:
        throw new Error(`Unknown argument: ${arg}`);
    }
  }
  return args;
}

async function readStdin(): Promise<string> {
  const chunks: Buffer[] = [];
  for await (const chunk of process.stdin) {
    chunks.push(chunk as Buffer);
  }
  return Buffer.concat(chunks).toString('utf8');
}

// --- XML extraction (mirrors xml_text_overlap_lint.py) ---

function stripXml(value: string, preserveLineBreaks: boolean): string {
  let stripped = value.replace(/<br\s*\/?>/gi, '\n').replace(/<[^>]+>/g, '');
  stripped = stripped
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'");
  if (preserveLineBreaks) {
    return stripped
      .split('\n')
      .map(line => line.replace(/\s+/g, ' ').trim())
      .join('\n');
  }
  return stripped.replace(/\s+/g, ' ').trim();
}

function extractAttribute(attrs: string, name: string): string | null {
  const match = attrs.match(new RegExp(`\\b${name}\\s*=\\s*"([^"]*)"`, 'i'));
  return match ? match[1] : null;
}

function extractNumericAttribute(attrs: string, name: string): number | null {
  const raw = extractAttribute(attrs, name);
  if (raw === null) {
    return null;
  }
  const value = parseFloat(raw);
  return Number.isFinite(value) ? value : null;
}

function extractBoolAttribute(attrs: string, name: string): boolean {
  return extractAttribute(attrs, name) === 'true';
}

// Largest span fontSize wins, falling back to the content default (mirrors
// extract_max_span_font_size).
function extractMaxSpanFontSize(body: string, defaultFontSize: number): number {
  const sizes = [defaultFontSize];
  for (const [, spanAttrs] of body.matchAll(/<span\b([^>]*)>/gi)) {
    const size = extractNumericAttribute(spanAttrs, 'fontSize');
    if (size !== null) {
      sizes.push(size);
    }
  }
  return Math.max(...sizes);
}

function detectAnySpanBool(body: string, name: string): boolean {
  for (const [, spanAttrs] of body.matchAll(/<span\b([^>]*)>/gi)) {
    if (extractBoolAttribute(spanAttrs, name)) {
      return true;
    }
  }
  return false;
}

// Extract one segment per <p> inside each <shape ...><content ...>. Each hard
// paragraph is measured independently (single-line scope).
function extractSegments(xml: string): Segment[] {
  const segments: Segment[] = [];
  for (const [, shapeAttrs, shapeBody] of xml.matchAll(/<shape\b([^>]*)>([\s\S]*?)<\/shape\s*>/gi)) {
    const shapeId = extractAttribute(shapeAttrs, 'id');
    const contentMatch = shapeBody.match(/<content\b([^>]*)>([\s\S]*?)<\/content\s*>/i);
    if (!contentMatch) {
      continue;
    }
    const [, contentAttrs, contentBody] = contentMatch;
    const contentFontSize = extractNumericAttribute(contentAttrs, 'fontSize') ?? 16;
    const contentFontFamily = extractAttribute(contentAttrs, 'fontFamily') ?? '';
    const contentLetterSpacing = extractNumericAttribute(contentAttrs, 'letterSpacing');
    const contentBold = extractBoolAttribute(contentAttrs, 'bold');
    const contentItalic = extractBoolAttribute(contentAttrs, 'italic');

    const paragraphs = [...contentBody.matchAll(/<p\b([^>]*)>([\s\S]*?)<\/p\s*>/gi)];
    const iter = paragraphs.length > 0 ? paragraphs : [['', '', contentBody] as unknown as RegExpMatchArray];

    for (const [, , body] of iter) {
      const text = stripXml(body, true);
      if (!text) {
        continue;
      }
      const fontSize = extractMaxSpanFontSize(body, contentFontSize);
      const bold = contentBold || detectAnySpanBool(body, 'bold');
      const italic = contentItalic || detectAnySpanBool(body, 'italic');
      const letterSpacing = contentLetterSpacing ?? 0;
      segments.push({
        shapeId,
        text,
        style: {
          fontSize: `${fontSize}px`,
          fontFamily: contentFontFamily,
          letterSpacing: `${letterSpacing}px`,
          bold,
          italic,
        },
      });
    }
  }
  return segments;
}

async function main(): Promise<void> {
  const args = parseArgs(process.argv.slice(2));

  const runtime = new StandaloneMeasureRuntime();
  await runtime.ensureReady(args.fonts);

  // Each segment may itself contain hard line breaks; measure each visual line
  // and report the max width (single-line scope: no wrapping).
  const measureSegment = (text: string, style: SingleLineStyle) => {
    const lines = text.split('\n');
    const widths = lines.map(line => measureSingleLineText(line, style, runtime));
    return { widthPx: Math.max(...widths, 0), lineWidths: widths };
  };

  let output: unknown;

  if (args.text !== undefined) {
    const style: SingleLineStyle = {
      fontSize: args.fontSize,
      fontFamily: args.fontFamily,
      letterSpacing: args.letterSpacing,
      bold: args.bold,
      italic: args.italic,
    };
    const { widthPx, lineWidths } = measureSegment(args.text, style);
    output = { mode: 'text', text: args.text, style, widthPx, lineWidths };
  } else {
    const xml = args.xml ?? (args.file ? await readFile(args.file, 'utf8') : await readStdin());
    if (!xml.trim()) {
      throw new Error('No XML input provided (use --file, --xml, --text, or pipe via stdin).');
    }
    const segments = extractSegments(xml);
    output = {
      mode: 'xml',
      segmentCount: segments.length,
      segments: segments.map(seg => {
        const { widthPx, lineWidths } = measureSegment(seg.text, seg.style);
        return { shapeId: seg.shapeId, text: seg.text, style: seg.style, widthPx, lineWidths };
      }),
    };
  }

  runtime.dispose();
  process.stdout.write(JSON.stringify(output, null, 2) + '\n');
}

main().catch((error: unknown) => {
  process.stderr.write(`${error instanceof Error ? error.stack ?? error.message : String(error)}\n`);
  process.exit(1);
});
