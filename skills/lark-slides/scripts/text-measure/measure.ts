// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT
//
// Core single-line text measurement, transcribed verbatim from ee/slide
// modules/text-measure-module/src/util/canvaskit/text-layout.ts
// (function measureSingleLineText, lines 1717-1776) so that the width produced
// here is identical to what the Slides renderer computes on-line.
//
// Only the single-line path is reproduced (per the agreed scope). The multi-run
// measureTextWithCanvasKit path — which needs an editor-kit ZoneDelta — is not
// included here.

import type { CanvasKit, FontCollection, TypefaceFontProvider } from 'canvaskit-wasm';

/**
 * Read-only runtime view the measure function depends on. Mirrors
 * CanvasKitMeasureRuntime in text-measure-module/src/type/canvaskit.type.ts.
 * Initialization (WASM load, font registration) is the runtime's job, not the
 * measure function's.
 */
export interface CanvasKitMeasureRuntime {
  readonly getCanvasKit: () => CanvasKit | null;
  readonly getFontProvider: () => TypefaceFontProvider | null;
  readonly getFontCollection: () => FontCollection | null;
  readonly getDefaultFontFamily: () => string;
  readonly isFontRegistered: (fontFamily: string) => boolean;
  readonly hasWghtAxis: (fontFamily: string) => boolean;
}

export interface SingleLineStyle {
  fontSize: string;
  fontFamily: string;
  letterSpacing: string;
  bold: boolean;
  italic: boolean;
}

/**
 * Verbatim transcription of measureSingleLineText from text-layout.ts:1717.
 * Returns the natural (unwrapped) width in px via Skia's getMaxIntrinsicWidth.
 */
export function measureSingleLineText(
  text: string,
  style: SingleLineStyle,
  runtime: CanvasKitMeasureRuntime,
): number {
  const ck = runtime.getCanvasKit();
  const fontProvider = runtime.getFontProvider();
  if (!ck || !fontProvider) {
    throw new Error('CanvasKit runtime not ready, call ensureReady() before measuring');
  }

  const fontSize = parseInt(style.fontSize, 10) || 16;
  const letterSpacing = parseFloat(style.letterSpacing) || 0;
  const rawFontFamily = style.fontFamily.split(',')[0].trim().replace(/['"]/g, '');
  const defaultFont = runtime.getDefaultFontFamily();
  const fontFamilies = runtime.isFontRegistered(rawFontFamily) ? [rawFontFamily, defaultFont] : [defaultFont];

  const textStyle = new ck.TextStyle({
    fontSize,
    fontFamilies,
    letterSpacing,
    fontStyle: {
      weight: style.bold ? ck.FontWeight.Bold : ck.FontWeight.Normal,
      slant: style.italic ? ck.FontSlant.Italic : ck.FontSlant.Upright,
    },
  });

  const paraStyle = new ck.ParagraphStyle({
    textStyle: {
      fontSize,
      fontFamilies,
    },
  });

  const builder = ck.ParagraphBuilder.MakeFromFontProvider(paraStyle, fontProvider);

  try {
    builder.pushStyle(textStyle);
    builder.addText(text);
    builder.pop();

    const paragraph = builder.build();

    try {
      // Unbounded width: measure the full natural width of a single line.
      paragraph.layout(Infinity);
      return paragraph.getMaxIntrinsicWidth();
    } finally {
      paragraph.delete();
    }
  } finally {
    builder.delete();
  }
}
