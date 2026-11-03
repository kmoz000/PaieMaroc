// Chargement des documents (PDF, DOCX, image), composition de la signature
// et production du PDF final à enregistrer ou imprimer. Tout se passe dans
// le navigateur embarqué : aucun document ne quitte le poste.

import { GlobalWorkerOptions, getDocument } from "pdfjs-dist";
import pdfWorkerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import { PDFDocument } from "pdf-lib";

GlobalWorkerOptions.workerSrc = pdfWorkerUrl;

export type DocumentKind = "pdf" | "docx" | "image";

export type LoadedDocument = {
  kind: DocumentKind;
  fileName: string;
  /** Pages rendues pour l'aperçu et l'aplatissement, au ratio A4. */
  pages: HTMLCanvasElement[];
  /** Octets du PDF d'origine, pour tamponner sans perte quand c'en est un. */
  pdfBytes: Uint8Array | null;
  /** Avertissement non bloquant (fidélité DOCX, repli de rendu…). */
  warning: string;
};

export type SignaturePlacement = {
  pageIndex: number;
  /** Fractions de la page : x,y = coin supérieur gauche, width = largeur. */
  x: number;
  y: number;
  width: number;
};

export const A4_WIDTH_POINTS = 595.28;
export const A4_HEIGHT_POINTS = 841.89;
export const A4_RATIO = A4_HEIGHT_POINTS / A4_WIDTH_POINTS;

const PREVIEW_WIDTH = 1000;
const PRINT_WIDTH = 1654; // ≈ 200 ppp sur une largeur A4.

export function detectKind(fileName: string, mimeType: string): DocumentKind | null {
  const name = fileName.toLowerCase();
  if (name.endsWith(".pdf") || mimeType === "application/pdf") return "pdf";
  if (
    name.endsWith(".docx") ||
    mimeType ===
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
  )
    return "docx";
  if (/\.(png|jpe?g|webp|bmp|gif)$/.test(name) || mimeType.startsWith("image/"))
    return "image";
  return null;
}

export async function loadDocument(file: File): Promise<LoadedDocument> {
  const kind = detectKind(file.name, file.type || "");
  if (!kind) {
    throw new Error(
      "Format non pris en charge. Choisissez un PDF, un DOCX ou une image.",
    );
  }
  if (kind === "pdf") return loadPDF(file);
  if (kind === "docx") return loadDOCX(file);
  return loadImage(file);
}

async function loadPDF(file: File): Promise<LoadedDocument> {
  const bytes = new Uint8Array(await file.arrayBuffer());
  const pages = await renderPDFPages(bytes.slice(), PREVIEW_WIDTH);
  return { kind: "pdf", fileName: file.name, pages, pdfBytes: bytes, warning: "" };
}

async function renderPDFPages(
  bytes: Uint8Array,
  targetWidth: number,
): Promise<HTMLCanvasElement[]> {
  const loadingTask = getDocument({ data: bytes });
  const document = await loadingTask.promise;
  const pages: HTMLCanvasElement[] = [];
  try {
    for (let number = 1; number <= document.numPages; number++) {
      const page = await document.getPage(number);
      const baseViewport = page.getViewport({ scale: 1 });
      const scale = targetWidth / baseViewport.width;
      const viewport = page.getViewport({ scale });
      const canvas = window.document.createElement("canvas");
      canvas.width = Math.round(viewport.width);
      canvas.height = Math.round(viewport.height);
      const context = canvas.getContext("2d");
      if (!context) throw new Error("Canvas indisponible pour le rendu PDF.");
      context.fillStyle = "#ffffff";
      context.fillRect(0, 0, canvas.width, canvas.height);
      await page.render({ canvas, canvasContext: context, viewport }).promise;
      pages.push(canvas);
      page.cleanup();
    }
  } finally {
    await loadingTask.destroy();
  }
  return pages;
}

async function loadImage(file: File): Promise<LoadedDocument> {
  const bitmap = await readImage(file);
  const canvas = document.createElement("canvas");
  canvas.width = PRINT_WIDTH;
  canvas.height = Math.round(PRINT_WIDTH * A4_RATIO);
  const context = canvas.getContext("2d");
  if (!context) throw new Error("Canvas indisponible pour le rendu de l'image.");
  context.fillStyle = "#ffffff";
  context.fillRect(0, 0, canvas.width, canvas.height);
  const margin = Math.round(canvas.width * 0.04);
  const availableWidth = canvas.width - margin * 2;
  const availableHeight = canvas.height - margin * 2;
  const scale = Math.min(
    availableWidth / bitmap.width,
    availableHeight / bitmap.height,
  );
  const width = bitmap.width * scale;
  const height = bitmap.height * scale;
  context.drawImage(
    bitmap,
    margin + (availableWidth - width) / 2,
    margin + (availableHeight - height) / 2,
    width,
    height,
  );
  return {
    kind: "image",
    fileName: file.name,
    pages: [canvas],
    pdfBytes: null,
    warning: "",
  };
}

export async function readImage(file: Blob): Promise<HTMLImageElement> {
  const url = URL.createObjectURL(file);
  try {
    return await loadImageURL(url);
  } finally {
    URL.revokeObjectURL(url);
  }
}

export function loadImageURL(url: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const image = new Image();
    image.onload = () => resolve(image);
    image.onerror = () =>
      reject(new Error("Impossible de lire cette image."));
    image.src = url;
  });
}

async function loadDOCX(file: File): Promise<LoadedDocument> {
  const [{ renderAsync }, html2canvasModule] = await Promise.all([
    import("docx-preview"),
    import("html2canvas"),
  ]);
  const html2canvas = html2canvasModule.default;
  const host = document.createElement("div");
  host.style.position = "fixed";
  host.style.left = "-100000px";
  host.style.top = "0";
  host.style.width = "900px";
  host.style.background = "#ffffff";
  document.body.appendChild(host);
  try {
    await renderAsync(await file.arrayBuffer(), host, undefined, {
      inWrapper: true,
      ignoreWidth: false,
      ignoreHeight: false,
      breakPages: true,
    });
    const sections = Array.from(
      host.querySelectorAll<HTMLElement>("section.docx"),
    );
    if (sections.length === 0) {
      throw new Error("Ce document DOCX n'a produit aucune page lisible.");
    }
    const pages: HTMLCanvasElement[] = [];
    for (const section of sections) {
      const rendered = await html2canvas(section, {
        scale: PRINT_WIDTH / Math.max(1, section.offsetWidth),
        backgroundColor: "#ffffff",
        logging: false,
      });
      pages.push(normalizeToA4(rendered));
    }
    return {
      kind: "docx",
      fileName: file.name,
      pages,
      pdfBytes: null,
      warning:
        "Aperçu DOCX approché : vérifiez la mise en page avant d'imprimer.",
    };
  } finally {
    host.remove();
  }
}

/** Replace un rendu quelconque sur une page A4 blanche, sans déformation. */
function normalizeToA4(source: HTMLCanvasElement): HTMLCanvasElement {
  const canvas = document.createElement("canvas");
  canvas.width = PRINT_WIDTH;
  canvas.height = Math.round(PRINT_WIDTH * A4_RATIO);
  const context = canvas.getContext("2d");
  if (!context) return source;
  context.fillStyle = "#ffffff";
  context.fillRect(0, 0, canvas.width, canvas.height);
  const scale = Math.min(
    canvas.width / source.width,
    canvas.height / source.height,
  );
  const width = source.width * scale;
  const height = source.height * scale;
  context.drawImage(source, (canvas.width - width) / 2, 0, width, height);
  return canvas;
}

/**
 * Construit le PDF signé. Un PDF d'origine est tamponné sans perte; les
 * autres formats sont aplatis en pages A4.
 */
export async function buildSignedPDF(
  loaded: LoadedDocument,
  signaturePNG: Uint8Array,
  placements: SignaturePlacement[],
): Promise<Uint8Array> {
  if (loaded.kind === "pdf" && loaded.pdfBytes) {
    return stampPDF(loaded.pdfBytes, signaturePNG, placements);
  }
  const output = await PDFDocument.create();
  const signatureImage = await output.embedPng(signaturePNG);
  for (let index = 0; index < loaded.pages.length; index++) {
    const canvas = loaded.pages[index];
    const page = output.addPage([A4_WIDTH_POINTS, A4_HEIGHT_POINTS]);
    const jpeg = await output.embedJpg(
      dataURLToBytes(canvas.toDataURL("image/jpeg", 0.92)),
    );
    page.drawImage(jpeg, {
      x: 0,
      y: 0,
      width: A4_WIDTH_POINTS,
      height: A4_HEIGHT_POINTS,
    });
    drawPlacements(page, signatureImage, placements, index);
  }
  return output.save();
}

async function stampPDF(
  bytes: Uint8Array,
  signaturePNG: Uint8Array,
  placements: SignaturePlacement[],
): Promise<Uint8Array> {
  const documentPDF = await PDFDocument.load(bytes.slice(), {
    ignoreEncryption: false,
  });
  const signatureImage = await documentPDF.embedPng(signaturePNG);
  const pages = documentPDF.getPages();
  for (let index = 0; index < pages.length; index++) {
    drawPlacements(pages[index], signatureImage, placements, index);
  }
  return documentPDF.save();
}

type EmbeddedImage = Awaited<ReturnType<PDFDocument["embedPng"]>>;
type PDFPage = ReturnType<PDFDocument["addPage"]>;

function drawPlacements(
  page: PDFPage,
  image: EmbeddedImage,
  placements: SignaturePlacement[],
  pageIndex: number,
) {
  const { width: pageWidth, height: pageHeight } = page.getSize();
  for (const placement of placements) {
    if (placement.pageIndex !== pageIndex) continue;
    const width = placement.width * pageWidth;
    const height = width * (image.height / image.width);
    page.drawImage(image, {
      x: placement.x * pageWidth,
      // pdf-lib compte depuis le bas de page.
      y: pageHeight - placement.y * pageHeight - height,
      width,
      height,
    });
  }
}

export function dataURLToBytes(dataURL: string): Uint8Array {
  const base64 = dataURL.slice(dataURL.indexOf(",") + 1);
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let index = 0; index < binary.length; index++) {
    bytes[index] = binary.charCodeAt(index);
  }
  return bytes;
}

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = "";
  const chunk = 0x8000;
  for (let offset = 0; offset < bytes.length; offset += chunk) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunk));
  }
  return btoa(binary);
}

/**
 * Rastérise le PDF signé pour l'impression Windows : JPEG ~150 ppp, page
 * par page, pour limiter la mémoire et la taille du transfert vers Go.
 */
export async function rasterizeForPrint(pdfBytes: Uint8Array): Promise<string[]> {
  const targetWidth = 1240; // ≈ 150 ppp sur une largeur A4.
  const loadingTask = getDocument({ data: pdfBytes.slice() });
  const documentPDF = await loadingTask.promise;
  const pages: string[] = [];
  try {
    for (let number = 1; number <= documentPDF.numPages; number++) {
      const page = await documentPDF.getPage(number);
      const baseViewport = page.getViewport({ scale: 1 });
      const viewport = page.getViewport({ scale: targetWidth / baseViewport.width });
      const canvas = window.document.createElement("canvas");
      canvas.width = Math.round(viewport.width);
      canvas.height = Math.round(viewport.height);
      const context = canvas.getContext("2d");
      if (!context) throw new Error("Canvas indisponible pour l'impression.");
      context.fillStyle = "#ffffff";
      context.fillRect(0, 0, canvas.width, canvas.height);
      await page.render({ canvas, canvasContext: context, viewport }).promise;
      const dataURL = canvas.toDataURL("image/jpeg", 0.9);
      pages.push(dataURL.slice(dataURL.indexOf(",") + 1));
      page.cleanup();
      canvas.width = 0;
      canvas.height = 0;
    }
  } finally {
    await loadingTask.destroy();
  }
  return pages;
}
