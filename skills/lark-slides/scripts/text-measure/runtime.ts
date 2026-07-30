// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT
//
// Standalone CanvasKit runtime that satisfies CanvasKitMeasureRuntime without
// the @slide/modular DI container. The CanvasKit init and font-provider setup
// are distilled from ee/slide text-measure-module:
//   - createCanvasKit()      <- provide/canvaskit/canvaskit.service.implement.ts:64
//   - TypefaceFontProvider   <- provide/canvaskit-font/canvaskit-font.service.base.ts:67
//   - registerFont()         <- canvaskit-font.service.base.ts:108
//   - Node wasm path resolve <- provide/canvaskit/canvaskit.node.util.ts:6

import { createRequire } from 'node:module';
import { readFile } from 'node:fs/promises';

import type { CanvasKit, FontCollection, TypefaceFontProvider } from 'canvaskit-wasm';
import type { CanvasKitMeasureRuntime } from './measure.ts';

const nodeRequire = createRequire(import.meta.url);

function resolveNodeCanvasKitWasmPath(): string {
  return nodeRequire.resolve('canvaskit-wasm/bin/canvaskit.wasm');
}

export interface FontSpec {
  /** font-family name the XML references (matched by measureSingleLineText). */
  family: string;
  /** absolute path to the font file (.ttf/.otf/.ttc). */
  path: string;
  /** true for variable fonts exposing a `wght` axis. */
  hasWghtAxis?: boolean;
}

/**
 * Minimal runtime: loads CanvasKit WASM, creates one TypefaceFontProvider, and
 * registers caller-supplied font files. The first registered font becomes the
 * default family (mirrors registerDefaultFont in the base service).
 */
export class StandaloneMeasureRuntime implements CanvasKitMeasureRuntime {
  private canvasKit: CanvasKit | null = null;
  private fontProvider: TypefaceFontProvider | null = null;
  private fontCollection: FontCollection | null = null;
  private readonly registeredFonts = new Set<string>();
  private readonly wghtAxisFonts = new Set<string>();
  private defaultFontFamily = 'sans-serif';

  async ensureReady(fonts: readonly FontSpec[]): Promise<void> {
    if (!this.canvasKit) {
      const canvasKitModule = await import('canvaskit-wasm');
      const wasmUrl = resolveNodeCanvasKitWasmPath();
      this.canvasKit = await canvasKitModule.default({
        locateFile: (file: string) => (file.endsWith('.wasm') ? wasmUrl : file),
      });
    }

    if (!this.fontProvider) {
      this.fontProvider = this.canvasKit.TypefaceFontProvider.Make();
    }

    if (!this.fontCollection) {
      this.fontCollection = this.canvasKit.FontCollection.Make();
      this.fontCollection.setDefaultFontManager(this.fontProvider);
      this.fontCollection.enableFontFallback();
    }

    for (const [index, font] of fonts.entries()) {
      await this.registerFont(font, index === 0);
    }

    if (!this.hasAnyRegisteredFont()) {
      // CanvasKit embeds no fonts; with none registered Skia finds no glyphs and
      // measures 0 width while resolving successfully — worse than failing.
      throw new Error('No font registered; measurement would be unreliable. Pass at least one --font family=path.');
    }
  }

  private async registerFont(font: FontSpec, isDefault: boolean): Promise<void> {
    if (!this.fontProvider) {
      throw new Error('Font provider not initialized');
    }
    const data = await readFile(font.path);
    const buffer = new Uint8Array(data.buffer, data.byteOffset, data.byteLength);
    const result = this.fontProvider.registerFont(buffer, font.family) as null | undefined;
    if (result === null) {
      console.warn(`[CanvasKit] registerFont failed for ${font.family} (${font.path})`);
      return;
    }
    this.registeredFonts.add(font.family);
    if (font.hasWghtAxis) {
      this.wghtAxisFonts.add(font.family);
    }
    if (isDefault) {
      this.defaultFontFamily = font.family;
    }
  }

  hasAnyRegisteredFont(): boolean {
    return this.registeredFonts.size > 0;
  }

  getCanvasKit(): CanvasKit | null {
    return this.canvasKit;
  }

  getFontProvider(): TypefaceFontProvider | null {
    return this.fontProvider;
  }

  getFontCollection(): FontCollection | null {
    return this.fontCollection;
  }

  getDefaultFontFamily(): string {
    return this.defaultFontFamily;
  }

  isFontRegistered(fontFamily: string): boolean {
    return this.registeredFonts.has(fontFamily);
  }

  hasWghtAxis(fontFamily: string): boolean {
    return this.wghtAxisFonts.has(fontFamily);
  }

  dispose(): void {
    this.fontCollection?.delete();
    this.fontProvider?.delete();
    this.fontCollection = null;
    this.fontProvider = null;
    this.canvasKit = null;
    this.registeredFonts.clear();
    this.wghtAxisFonts.clear();
  }
}
