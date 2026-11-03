// Nettoyage automatique des signatures scannées ou photographiées :
// normalisation de l'éclairage, seuillage d'Otsu à bord doux, suppression
// des taches, lissage et ré-encrage. Fonctions pures sur des tampons de
// pixels, testables sans canvas.

export type RawImage = {
  data: Uint8ClampedArray;
  width: number;
  height: number;
};

export type InkMode = "noir" | "bleu" | "original";

export type CleanupOptions = {
  /** Décalage du seuil d'encre, -50 (moins d'encre) à +50 (plus d'encre). */
  thresholdBias: number;
  /** Passes de lissage du contour, 0 à 3. */
  smoothing: number;
  /** Épaisseur du trait, -2 (plus fin) à +2 (plus épais). */
  thickness: number;
  ink: InkMode;
};

export type CleanupResult = {
  image: RawImage;
  inkColor: [number, number, number];
  inkPixels: number;
};

export const defaultCleanupOptions: CleanupOptions = {
  thresholdBias: 0,
  smoothing: 1,
  thickness: 0,
  ink: "noir",
};

const BLACK_INK: [number, number, number] = [24, 26, 34];
const BLUE_INK: [number, number, number] = [23, 52, 134];

export function grayscale(image: RawImage): Float32Array {
  const { data, width, height } = image;
  const gray = new Float32Array(width * height);
  for (let index = 0; index < gray.length; index++) {
    const offset = index * 4;
    gray[index] =
      0.299 * data[offset] + 0.587 * data[offset + 1] + 0.114 * data[offset + 2];
  }
  return gray;
}

/** Flou en boîte séparable avec sommes glissantes, O(n) par passe. */
export function boxBlur(
  source: Float32Array,
  width: number,
  height: number,
  radius: number,
): Float32Array {
  const clampedRadius = Math.max(1, Math.round(radius));
  const horizontal = new Float32Array(source.length);
  for (let y = 0; y < height; y++) {
    const row = y * width;
    let sum = 0;
    for (let x = -clampedRadius; x <= clampedRadius; x++) {
      sum += source[row + clampIndex(x, width)];
    }
    const window = clampedRadius * 2 + 1;
    for (let x = 0; x < width; x++) {
      horizontal[row + x] = sum / window;
      sum += source[row + clampIndex(x + clampedRadius + 1, width)];
      sum -= source[row + clampIndex(x - clampedRadius, width)];
    }
  }
  const vertical = new Float32Array(source.length);
  for (let x = 0; x < width; x++) {
    let sum = 0;
    for (let y = -clampedRadius; y <= clampedRadius; y++) {
      sum += horizontal[clampIndex(y, height) * width + x];
    }
    const window = clampedRadius * 2 + 1;
    for (let y = 0; y < height; y++) {
      vertical[y * width + x] = sum / window;
      sum += horizontal[clampIndex(y + clampedRadius + 1, height) * width + x];
      sum -= horizontal[clampIndex(y - clampedRadius, height) * width + x];
    }
  }
  return vertical;
}

function clampIndex(value: number, limit: number): number {
  if (value < 0) return 0;
  if (value >= limit) return limit - 1;
  return value;
}

/**
 * Normalise l'éclairage en divisant chaque pixel par le fond local estimé,
 * ce qui neutralise les ombres des photos de téléphone. Retourne des
 * valeurs centrées autour de 128 pour le fond.
 */
export function normalizeIllumination(
  gray: Float32Array,
  width: number,
  height: number,
): Float32Array {
  // Le rayon doit dépasser la largeur d'un trait pour que le fond local ne
  // soit pas contaminé par l'encre elle-même.
  const radius = Math.max(12, Math.round(Math.max(width, height) * 0.08));
  const background = boxBlur(gray, width, height, radius);
  const normalized = new Float32Array(gray.length);
  for (let index = 0; index < gray.length; index++) {
    const reference = Math.max(background[index], 1);
    normalized[index] = Math.min(255, (gray[index] / reference) * 128);
  }
  return normalized;
}

/** Seuil d'Otsu sur un tampon de valeurs 0..255. */
export function otsuThreshold(values: Float32Array): number {
  const histogram = new Float64Array(256);
  for (let index = 0; index < values.length; index++) {
    histogram[Math.max(0, Math.min(255, Math.round(values[index])))]++;
  }
  const total = values.length;
  let sumAll = 0;
  for (let level = 0; level < 256; level++) sumAll += level * histogram[level];
  let sumBackground = 0;
  let weightBackground = 0;
  let bestVariance = -1;
  let threshold = 127;
  for (let level = 0; level < 256; level++) {
    weightBackground += histogram[level];
    if (weightBackground === 0) continue;
    const weightForeground = total - weightBackground;
    if (weightForeground === 0) break;
    sumBackground += level * histogram[level];
    const meanBackground = sumBackground / weightBackground;
    const meanForeground = (sumAll - sumBackground) / weightForeground;
    const difference = meanBackground - meanForeground;
    const variance = weightBackground * weightForeground * difference * difference;
    if (variance > bestVariance) {
      bestVariance = variance;
      threshold = level;
    }
  }
  return threshold;
}

/**
 * Centre de coupure : à mi-chemin entre la moyenne de l'encre et celle du
 * fond. Le seuil d'Otsu brut peut tomber sur le mode encre lui-même, ce qui
 * rendrait le trait semi-transparent.
 */
export function sigmoidCenter(values: Float32Array, threshold: number): number {
  let sumLow = 0;
  let countLow = 0;
  let sumHigh = 0;
  let countHigh = 0;
  for (let index = 0; index < values.length; index++) {
    if (values[index] <= threshold) {
      sumLow += values[index];
      countLow++;
    } else {
      sumHigh += values[index];
      countHigh++;
    }
  }
  if (countLow === 0 || countHigh === 0) return threshold;
  return (sumLow / countLow + sumHigh / countHigh) / 2;
}

/** Alpha doux autour du seuil (sigmoïde) : garde des bords lisses. */
export function softAlpha(
  normalized: Float32Array,
  threshold: number,
  bias: number,
): Float32Array {
  const center = threshold + bias;
  const softness = 7;
  const alpha = new Float32Array(normalized.length);
  for (let index = 0; index < normalized.length; index++) {
    alpha[index] = 255 / (1 + Math.exp((normalized[index] - center) / softness));
  }
  return alpha;
}

/**
 * Supprime les petites taches et les ombres de bord de scan : composantes
 * connexes de moins de 0,02 % de l'image, et grandes bandes collées au bord.
 */
export function removeSpecks(
  alpha: Float32Array,
  width: number,
  height: number,
): Float32Array {
  const total = width * height;
  const minArea = Math.max(6, Math.round(total * 0.0002));
  const labels = new Int32Array(total);
  const stack = new Int32Array(total);
  const cleaned = Float32Array.from(alpha);
  let label = 0;
  for (let start = 0; start < total; start++) {
    if (labels[start] !== 0 || alpha[start] < 96) continue;
    label++;
    let stackSize = 0;
    stack[stackSize++] = start;
    labels[start] = label;
    const pixels: number[] = [];
    let touchesBorder = false;
    while (stackSize > 0) {
      const index = stack[--stackSize];
      pixels.push(index);
      const x = index % width;
      const y = (index / width) | 0;
      if (x === 0 || y === 0 || x === width - 1 || y === height - 1) {
        touchesBorder = true;
      }
      const neighbors = [
        x > 0 ? index - 1 : -1,
        x < width - 1 ? index + 1 : -1,
        y > 0 ? index - width : -1,
        y < height - 1 ? index + width : -1,
      ];
      for (const neighbor of neighbors) {
        if (neighbor >= 0 && labels[neighbor] === 0 && alpha[neighbor] >= 96) {
          labels[neighbor] = label;
          stack[stackSize++] = neighbor;
        }
      }
    }
    const isSpeck = pixels.length < minArea;
    const isBorderShadow =
      touchesBorder && pixels.length > total * 0.08 && isHuggingBorder(pixels, width, height);
    if (isSpeck || isBorderShadow) {
      for (const pixel of pixels) cleaned[pixel] = 0;
    }
  }
  return cleaned;
}

function isHuggingBorder(pixels: number[], width: number, height: number): boolean {
  const band = Math.max(2, Math.round(Math.min(width, height) * 0.06));
  let inBand = 0;
  for (const index of pixels) {
    const x = index % width;
    const y = (index / width) | 0;
    if (x < band || y < band || x >= width - band || y >= height - band) inBand++;
  }
  return inBand / pixels.length > 0.6;
}

/** Dilatation (positif) ou érosion (négatif) 3×3 de l'alpha. */
export function adjustThickness(
  alpha: Float32Array,
  width: number,
  height: number,
  steps: number,
): Float32Array {
  let current = alpha;
  const passes = Math.min(2, Math.abs(Math.round(steps)));
  const pick = steps > 0 ? Math.max : Math.min;
  for (let pass = 0; pass < passes; pass++) {
    const next = new Float32Array(current.length);
    for (let y = 0; y < height; y++) {
      for (let x = 0; x < width; x++) {
        const index = y * width + x;
        let value = current[index];
        if (x > 0) value = pick(value, current[index - 1]);
        if (x < width - 1) value = pick(value, current[index + 1]);
        if (y > 0) value = pick(value, current[index - width]);
        if (y < height - 1) value = pick(value, current[index + width]);
        next[index] = value;
      }
    }
    current = next;
  }
  return current;
}

/** Lissage gaussien 3×3 de l'alpha (noyau 1-2-1). */
export function smoothAlpha(
  alpha: Float32Array,
  width: number,
  height: number,
  passes: number,
): Float32Array {
  let current = alpha;
  for (let pass = 0; pass < Math.max(0, Math.min(4, Math.round(passes))); pass++) {
    const horizontal = new Float32Array(current.length);
    for (let y = 0; y < height; y++) {
      const row = y * width;
      for (let x = 0; x < width; x++) {
        const left = current[row + clampIndex(x - 1, width)];
        const right = current[row + clampIndex(x + 1, width)];
        horizontal[row + x] = (left + 2 * current[row + x] + right) / 4;
      }
    }
    const vertical = new Float32Array(current.length);
    for (let x = 0; x < width; x++) {
      for (let y = 0; y < height; y++) {
        const up = horizontal[clampIndex(y - 1, height) * width + x];
        const down = horizontal[clampIndex(y + 1, height) * width + x];
        vertical[y * width + x] = (up + 2 * horizontal[y * width + x] + down) / 4;
      }
    }
    current = vertical;
  }
  return current;
}

/** Couleur médiane des pixels d'encre franche, pour le mode « originale ». */
export function medianInkColor(
  image: RawImage,
  alpha: Float32Array,
): [number, number, number] {
  const reds: number[] = [];
  const greens: number[] = [];
  const blues: number[] = [];
  for (let index = 0; index < alpha.length; index++) {
    if (alpha[index] > 190) {
      const offset = index * 4;
      reds.push(image.data[offset]);
      greens.push(image.data[offset + 1]);
      blues.push(image.data[offset + 2]);
    }
  }
  if (reds.length === 0) return BLACK_INK;
  return [median(reds), median(greens), median(blues)];
}

function median(values: number[]): number {
  values.sort((left, right) => left - right);
  return values[values.length >> 1];
}

/** Boîte englobante de l'encre avec marge; null si aucune encre. */
export function inkBounds(
  alpha: Float32Array,
  width: number,
  height: number,
  padding: number,
): { x: number; y: number; width: number; height: number } | null {
  let minX = width;
  let minY = height;
  let maxX = -1;
  let maxY = -1;
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      if (alpha[y * width + x] > 24) {
        if (x < minX) minX = x;
        if (x > maxX) maxX = x;
        if (y < minY) minY = y;
        if (y > maxY) maxY = y;
      }
    }
  }
  if (maxX < 0) return null;
  const x = Math.max(0, minX - padding);
  const y = Math.max(0, minY - padding);
  return {
    x,
    y,
    width: Math.min(width - 1, maxX + padding) - x + 1,
    height: Math.min(height - 1, maxY + padding) - y + 1,
  };
}

/**
 * Pipeline complet : retourne une image RGBA transparente, recadrée sur la
 * signature, à l'encre uniforme et aux bords lisses.
 */
export function cleanSignature(
  source: RawImage,
  options: CleanupOptions = defaultCleanupOptions,
): CleanupResult {
  const { width, height } = source;
  const gray = grayscale(source);
  const normalized = normalizeIllumination(gray, width, height);
  const threshold = sigmoidCenter(normalized, otsuThreshold(normalized));
  const bias = Math.max(-50, Math.min(50, options.thresholdBias)) * 0.35;
  let alpha = softAlpha(normalized, threshold, bias);
  alpha = removeSpecks(alpha, width, height);
  // Une « encre » qui couvre presque toute l'image est un fond mal séparé
  // (image vide ou uniforme), pas une signature.
  let covered = 0;
  for (let index = 0; index < alpha.length; index++) {
    if (alpha[index] > 128) covered++;
  }
  if (covered === 0 || covered / alpha.length > 0.45) {
    return {
      image: { data: new Uint8ClampedArray(4), width: 1, height: 1 },
      inkColor: options.ink === "bleu" ? BLUE_INK : BLACK_INK,
      inkPixels: 0,
    };
  }
  if (options.thickness !== 0) {
    alpha = adjustThickness(alpha, width, height, options.thickness);
  }
  alpha = smoothAlpha(alpha, width, height, 1 + options.smoothing);
  const detected = medianInkColor(source, alpha);
  const ink: [number, number, number] =
    options.ink === "noir" ? BLACK_INK : options.ink === "bleu" ? BLUE_INK : detected;
  const bounds = inkBounds(alpha, width, height, Math.round(Math.max(width, height) * 0.015));
  if (!bounds) {
    return {
      image: { data: new Uint8ClampedArray(4), width: 1, height: 1 },
      inkColor: ink,
      inkPixels: 0,
    };
  }
  const output = new Uint8ClampedArray(bounds.width * bounds.height * 4);
  let inkPixels = 0;
  for (let y = 0; y < bounds.height; y++) {
    for (let x = 0; x < bounds.width; x++) {
      const sourceIndex = (bounds.y + y) * width + (bounds.x + x);
      const value = alpha[sourceIndex];
      const offset = (y * bounds.width + x) * 4;
      // Encre uniforme sur tout le trait : aucune frange claire en bord.
      output[offset] = ink[0];
      output[offset + 1] = ink[1];
      output[offset + 2] = ink[2];
      output[offset + 3] = Math.round(value);
      if (value > 128) inkPixels++;
    }
  }
  return {
    image: { data: output, width: bounds.width, height: bounds.height },
    inkColor: ink,
    inkPixels,
  };
}

/** Taille maximale de travail : au-delà, l'image est réduite avant nettoyage. */
export const maxWorkingPixels = 2000;
