import { describe, expect, it } from "vitest";
import {
  adjustThickness,
  cleanSignature,
  defaultCleanupOptions,
  inkBounds,
  otsuThreshold,
  removeSpecks,
  smoothAlpha,
  type RawImage,
} from "./signature-image";

function solidImage(
  width: number,
  height: number,
  color: [number, number, number],
): RawImage {
  const data = new Uint8ClampedArray(width * height * 4);
  for (let index = 0; index < width * height; index++) {
    data[index * 4] = color[0];
    data[index * 4 + 1] = color[1];
    data[index * 4 + 2] = color[2];
    data[index * 4 + 3] = 255;
  }
  return { data, width, height };
}

function paintRectangle(
  image: RawImage,
  x: number,
  y: number,
  width: number,
  height: number,
  color: [number, number, number],
) {
  for (let row = y; row < y + height; row++) {
    for (let column = x; column < x + width; column++) {
      const offset = (row * image.width + column) * 4;
      image.data[offset] = color[0];
      image.data[offset + 1] = color[1];
      image.data[offset + 2] = color[2];
    }
  }
}

describe("otsuThreshold", () => {
  it("sépare deux populations bien distinctes", () => {
    const values = new Float32Array(200);
    values.fill(30, 0, 100);
    values.fill(220, 100, 200);
    const threshold = otsuThreshold(values);
    expect(threshold).toBeGreaterThanOrEqual(30);
    expect(threshold).toBeLessThan(220);
  });
});

describe("cleanSignature", () => {
  it("rend le fond transparent et garde le trait encré", () => {
    const image = solidImage(120, 80, [235, 232, 228]);
    paintRectangle(image, 20, 30, 70, 8, [40, 42, 60]);
    const result = cleanSignature(image, defaultCleanupOptions);
    expect(result.inkPixels).toBeGreaterThan(200);
    // L'image est recadrée autour du trait, donc bien plus petite que la source.
    expect(result.image.width).toBeLessThan(120);
    expect(result.image.height).toBeLessThan(80);
    // Les coins du recadrage restent transparents ou presque.
    const corner = result.image.data[3];
    expect(corner).toBeLessThan(64);
    // Le cœur du trait est opaque et à l'encre noire par défaut.
    const centerOffset =
      ((result.image.height >> 1) * result.image.width +
        (result.image.width >> 1)) * 4;
    expect(result.image.data[centerOffset + 3]).toBeGreaterThan(190);
    expect(result.image.data[centerOffset]).toBe(24);
  });

  it("supprime les petites taches isolées", () => {
    const image = solidImage(200, 120, [240, 240, 240]);
    paintRectangle(image, 30, 50, 120, 10, [30, 30, 30]);
    paintRectangle(image, 180, 20, 2, 2, [30, 30, 30]);
    const result = cleanSignature(image, defaultCleanupOptions);
    // La tache de 4 pixels disparaît : le recadrage ne l'atteint pas.
    expect(result.image.width).toBeLessThan(150);
  });

  it("respecte le mode d'encre bleue", () => {
    const image = solidImage(100, 60, [250, 250, 250]);
    paintRectangle(image, 10, 25, 80, 6, [20, 20, 20]);
    const result = cleanSignature(image, {
      ...defaultCleanupOptions,
      ink: "bleu",
    });
    expect(result.inkColor).toEqual([23, 52, 134]);
  });

  it("retourne une image vide quand il n'y a pas d'encre", () => {
    const image = solidImage(64, 64, [255, 255, 255]);
    const result = cleanSignature(image, defaultCleanupOptions);
    expect(result.inkPixels).toBe(0);
    expect(result.image.width).toBe(1);
  });
});

describe("removeSpecks", () => {
  it("efface une composante minuscule et garde la grande", () => {
    const width = 100;
    const height = 100;
    const alpha = new Float32Array(width * height);
    for (let x = 10; x < 60; x++)
      for (let y = 40; y < 46; y++) alpha[y * width + x] = 255;
    alpha[5 * width + 90] = 255;
    const cleaned = removeSpecks(alpha, width, height);
    expect(cleaned[5 * width + 90]).toBe(0);
    expect(cleaned[42 * width + 30]).toBe(255);
  });
});

describe("adjustThickness", () => {
  it("épaissit puis affine le trait", () => {
    const width = 20;
    const height = 20;
    const alpha = new Float32Array(width * height);
    alpha[10 * width + 10] = 255;
    const thicker = adjustThickness(alpha, width, height, 1);
    expect(thicker[10 * width + 11]).toBe(255);
    const thinner = adjustThickness(thicker, width, height, -1);
    expect(thinner[10 * width + 12]).toBe(0);
  });
});

describe("smoothAlpha", () => {
  it("adoucit les transitions franches", () => {
    const width = 16;
    const height = 1;
    const alpha = new Float32Array(width);
    alpha.fill(255, 8);
    const smoothed = smoothAlpha(alpha, width, height, 1);
    expect(smoothed[8]).toBeLessThan(255);
    expect(smoothed[8]).toBeGreaterThan(0);
  });
});

describe("inkBounds", () => {
  it("retourne null sans encre et une boîte avec marge sinon", () => {
    const width = 50;
    const height = 50;
    const empty = new Float32Array(width * height);
    expect(inkBounds(empty, width, height, 2)).toBeNull();
    empty[25 * width + 25] = 255;
    const bounds = inkBounds(empty, width, height, 2);
    expect(bounds).toEqual({ x: 23, y: 23, width: 5, height: 5 });
  });
});
