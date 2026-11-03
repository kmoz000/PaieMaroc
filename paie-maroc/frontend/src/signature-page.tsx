// Page « Signer un document » : chargement PDF/DOCX/image, studio de
// signature (dessin ou photo nettoyée), modèles réutilisables, impression
// sur l'imprimante par défaut et enregistrement en PDF. Tout est local.

import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import type { PointerEvent as ReactPointerEvent } from "react";
import {
  A4_RATIO,
  buildSignedPDF,
  bytesToBase64,
  dataURLToBytes,
  loadDocument,
  loadImageURL,
  rasterizeForPrint,
  type LoadedDocument,
  type SignaturePlacement,
} from "./signature-document";
import {
  cleanSignature,
  defaultCleanupOptions,
  maxWorkingPixels,
  type CleanupOptions,
  type InkMode,
  type RawImage,
} from "./signature-image";

const isNative = Boolean((window as unknown as { go?: unknown }).go);

type SignatureTemplate = {
  id: string;
  name: string;
  pngBase64: string;
  x: number;
  y: number;
  width: number;
  createdAt: string;
  updatedAt: string;
};

type Printer = {
  name: string;
  systemDefault: boolean;
  storedDefault: boolean;
};

type PrinterState = {
  printers: Printer[];
  defaultPrinter: string;
  configured: boolean;
};

type ActiveSignature = {
  dataURL: string;
  pixelWidth: number;
  pixelHeight: number;
};

type Backend = {
  GetPrinterState: () => Promise<PrinterState>;
  SetDefaultPrinter: (name: string) => Promise<PrinterState>;
  PrintSignedDocument: (request: {
    documentName: string;
    printer: string;
    copies: number;
    pdfBase64: string;
    pageImages: string[];
  }) => Promise<void>;
  ListSignatureTemplates: () => Promise<SignatureTemplate[]>;
  SaveSignatureTemplate: (
    template: Partial<SignatureTemplate>,
  ) => Promise<SignatureTemplate>;
  DeleteSignatureTemplate: (id: string) => Promise<void>;
  SaveSignedDocument: (
    name: string,
    contentBase64: string,
  ) => Promise<{ path: string }>;
};

async function backend(): Promise<Backend> {
  const bindings = await import("../wailsjs/go/main/App");
  return bindings as unknown as Backend;
}

async function nativePlatform(): Promise<string> {
  try {
    const runtime = await import("../wailsjs/runtime/runtime");
    const environment = await runtime.Environment();
    return environment.platform;
  } catch {
    return "";
  }
}

function friendly(error: unknown): string {
  if (typeof error === "string") return error;
  if (error instanceof Error) return error.message;
  return String(error);
}

const DEFAULT_PLACEMENT = { x: 0.6, y: 0.78, width: 0.26 };

export default function SignaturePage({
  notify,
}: {
  notify: (message: string) => void;
}) {
  const [document_, setDocument] = useState<LoadedDocument | null>(null);
  const [documentBusy, setDocumentBusy] = useState(false);
  const [pageIndex, setPageIndex] = useState(0);
  const [signature, setSignature] = useState<ActiveSignature | null>(null);
  const [placements, setPlacements] = useState<SignaturePlacement[]>([]);
  const [templates, setTemplates] = useState<SignatureTemplate[]>([]);
  const [templateChoice, setTemplateChoice] = useState("");
  const [templateName, setTemplateName] = useState("");
  const [printerState, setPrinterState] = useState<PrinterState | null>(null);
  const [printerDialog, setPrinterDialog] = useState(false);
  const [copies, setCopies] = useState(1);
  const [working, setWorking] = useState("");

  const refreshTemplates = useCallback(async () => {
    if (!isNative) return;
    try {
      const api = await backend();
      setTemplates(await api.ListSignatureTemplates());
    } catch (error) {
      notify(friendly(error));
    }
  }, [notify]);

  useEffect(() => {
    void refreshTemplates();
    if (!isNative) return;
    backend()
      .then((api) => api.GetPrinterState())
      .then((state) => {
        setPrinterState(state);
        if (!state.configured) setPrinterDialog(true);
      })
      .catch((error) => notify(friendly(error)));
  }, [notify, refreshTemplates]);

  async function openDocument(file: File | null) {
    if (!file) return;
    setDocumentBusy(true);
    try {
      const loaded = await loadDocument(file);
      setDocument(loaded);
      setPageIndex(0);
      setPlacements([]);
      if (loaded.warning) notify(loaded.warning);
      else notify(`Document chargé : ${loaded.fileName} (${loaded.pages.length} page${loaded.pages.length > 1 ? "s" : ""})`);
    } catch (error) {
      notify(friendly(error));
    } finally {
      setDocumentBusy(false);
    }
  }

  function useSignature(next: ActiveSignature) {
    setSignature(next);
    notify("Signature prête. Apposez-la sur la page voulue.");
  }

  function placeOnCurrentPage(position = DEFAULT_PLACEMENT) {
    if (!signature || !document_) return;
    setPlacements((current) => [
      ...current.filter((placement) => placement.pageIndex !== pageIndex),
      { pageIndex, ...position },
    ]);
  }

  function applyTemplate(id: string) {
    setTemplateChoice(id);
    const template = templates.find((item) => item.id === id);
    if (!template) return;
    void loadImageURL(`data:image/png;base64,${template.pngBase64}`).then(
      (image) => {
        setSignature({
          dataURL: `data:image/png;base64,${template.pngBase64}`,
          pixelWidth: image.width,
          pixelHeight: image.height,
        });
        if (document_) {
          setPlacements((current) => [
            ...current.filter((placement) => placement.pageIndex !== pageIndex),
            { pageIndex, x: template.x, y: template.y, width: template.width },
          ]);
        }
        notify(`Modèle « ${template.name} » appliqué.`);
      },
    );
  }

  async function saveTemplate() {
    if (!signature) return;
    const name = templateName.trim();
    if (!name) {
      notify("Donnez un nom au modèle avant de l'enregistrer.");
      return;
    }
    if (!isNative) {
      notify("Les modèles sont disponibles dans l'application bureau.");
      return;
    }
    try {
      const api = await backend();
      const placement =
        placements.find((item) => item.pageIndex === pageIndex) ||
        placements[placements.length - 1];
      const saved = await api.SaveSignatureTemplate({
        name,
        pngBase64: signature.dataURL.slice(signature.dataURL.indexOf(",") + 1),
        x: placement?.x ?? DEFAULT_PLACEMENT.x,
        y: placement?.y ?? DEFAULT_PLACEMENT.y,
        width: placement?.width ?? DEFAULT_PLACEMENT.width,
      });
      setTemplateName("");
      await refreshTemplates();
      setTemplateChoice(saved.id);
      notify(`Modèle « ${saved.name} » enregistré; il restera disponible.`);
    } catch (error) {
      notify(friendly(error));
    }
  }

  async function deleteTemplate() {
    if (!templateChoice || !isNative) return;
    const template = templates.find((item) => item.id === templateChoice);
    if (!template) return;
    if (!confirm(`Supprimer le modèle « ${template.name} » ?`)) return;
    try {
      const api = await backend();
      await api.DeleteSignatureTemplate(templateChoice);
      setTemplateChoice("");
      await refreshTemplates();
      notify(`Modèle « ${template.name} » supprimé.`);
    } catch (error) {
      notify(friendly(error));
    }
  }

  const signedName = useMemo(() => {
    if (!document_) return "document-signe.pdf";
    const base = document_.fileName.replace(/\.[^.]+$/, "");
    return `${base}-signe.pdf`;
  }, [document_]);

  async function produceSignedPDF(): Promise<Uint8Array | null> {
    if (!document_ || !signature || placements.length === 0) {
      notify(
        "Chargez un document, préparez une signature et apposez-la avant de continuer.",
      );
      return null;
    }
    return buildSignedPDF(
      document_,
      dataURLToBytes(signature.dataURL),
      placements,
    );
  }

  async function saveSigned() {
    setWorking("Préparation du PDF signé…");
    try {
      const bytes = await produceSignedPDF();
      if (!bytes) return;
      if (!isNative) {
        const blob = new Blob([bytes as BlobPart], { type: "application/pdf" });
        const url = URL.createObjectURL(blob);
        const anchor = window.document.createElement("a");
        anchor.href = url;
        anchor.download = signedName;
        anchor.click();
        URL.revokeObjectURL(url);
        notify("PDF signé téléchargé (mode démonstration).");
        return;
      }
      const api = await backend();
      const result = await api.SaveSignedDocument(signedName, bytesToBase64(bytes));
      notify(`Document signé enregistré : ${result.path}`);
    } catch (error) {
      notify(friendly(error));
    } finally {
      setWorking("");
    }
  }

  async function printSigned() {
    if (!isNative) {
      notify("L'impression est disponible dans l'application bureau.");
      return;
    }
    if (!printerState?.defaultPrinter) {
      setPrinterDialog(true);
      return;
    }
    setWorking("Préparation de l'impression…");
    try {
      const bytes = await produceSignedPDF();
      if (!bytes) return;
      const platform = await nativePlatform();
      const pageImages =
        platform === "windows" ? await rasterizeForPrint(bytes) : [];
      setWorking(`Envoi vers ${printerState.defaultPrinter}…`);
      const api = await backend();
      await api.PrintSignedDocument({
        documentName: signedName,
        printer: printerState.defaultPrinter,
        copies,
        pdfBase64: platform === "windows" ? "" : bytesToBase64(bytes),
        pageImages,
      });
      notify(
        `Document envoyé à l'imprimante ${printerState.defaultPrinter} (${copies} exemplaire${copies > 1 ? "s" : ""}).`,
      );
    } catch (error) {
      notify(friendly(error));
    } finally {
      setWorking("");
    }
  }

  async function chooseDefaultPrinter(name: string) {
    try {
      const api = await backend();
      const state = await api.SetDefaultPrinter(name);
      setPrinterState(state);
      setPrinterDialog(false);
      notify(`Imprimante par défaut : ${name}`);
    } catch (error) {
      notify(friendly(error));
    }
  }

  const currentPlacement =
    placements.find((item) => item.pageIndex === pageIndex) || null;

  return (
    <div className="signature-layout">
      <section className="panel signature-document-panel">
        <div className="signature-toolbar">
          <label className="button primary signature-file-button">
            <input
              type="file"
              accept=".pdf,.docx,image/*"
              onChange={(event) => {
                void openDocument(event.target.files?.[0] ?? null);
                event.target.value = "";
              }}
            />
            Ouvrir un document
          </label>
          <span className="signature-file-name">
            {document_ ? document_.fileName : "PDF, DOCX ou image (A4)"}
          </span>
          <span className="signature-toolbar-space" />
          {isNative && (
            <button
              className="button"
              onClick={() => setPrinterDialog(true)}
              title="Changer d'imprimante par défaut"
            >
              {printerState?.defaultPrinter
                ? `Imprimante : ${printerState.defaultPrinter}`
                : "Choisir une imprimante"}
            </button>
          )}
          <label className="signature-copies">
            Exemplaires
            <input
              type="number"
              min={1}
              max={9}
              value={copies}
              onChange={(event) =>
                setCopies(Math.max(1, Math.min(9, Number(event.target.value) || 1)))
              }
            />
          </label>
          <button
            className="button"
            disabled={Boolean(working) || !document_}
            onClick={() => void printSigned()}
          >
            Imprimer
          </button>
          <button
            className="button primary"
            disabled={Boolean(working) || !document_}
            onClick={() => void saveSigned()}
          >
            Enregistrer le PDF signé
          </button>
        </div>
        {working && <p className="signature-working">{working}</p>}
        {documentBusy ? (
          <div className="signature-empty">
            <span className="spinner" />
            Lecture du document…
          </div>
        ) : document_ ? (
          <>
            <PageCanvas
              page={document_.pages[pageIndex]}
              signature={signature}
              placement={currentPlacement}
              onPlacementChange={(next) =>
                setPlacements((current) => [
                  ...current.filter((item) => item.pageIndex !== pageIndex),
                  { pageIndex, ...next },
                ])
              }
              onPlacementRemove={() =>
                setPlacements((current) =>
                  current.filter((item) => item.pageIndex !== pageIndex),
                )
              }
            />
            <div className="signature-page-nav">
              <button
                className="button"
                disabled={pageIndex === 0}
                onClick={() => setPageIndex((value) => value - 1)}
              >
                ← Page précédente
              </button>
              <span>
                Page {pageIndex + 1} / {document_.pages.length}
                {placements.length > 0 &&
                  ` · ${placements.length} signature${placements.length > 1 ? "s" : ""} apposée${placements.length > 1 ? "s" : ""}`}
              </span>
              <button
                className="button"
                disabled={pageIndex >= document_.pages.length - 1}
                onClick={() => setPageIndex((value) => value + 1)}
              >
                Page suivante →
              </button>
              <span className="signature-toolbar-space" />
              <button
                className="button"
                disabled={!signature}
                onClick={() => placeOnCurrentPage()}
              >
                Apposer la signature ici
              </button>
            </div>
          </>
        ) : (
          <div className="signature-empty">
            Choisissez un document à signer : contrat, attestation, bulletin…
            <br />
            <small>
              Il sera présenté au format A4, prêt à enregistrer ou imprimer.
            </small>
          </div>
        )}
      </section>

      <aside className="signature-side">
        <section className="panel signature-templates-panel">
          <h3>Modèles de signature</h3>
          <div className="signature-template-row">
            <select
              value={templateChoice}
              onChange={(event) => applyTemplate(event.target.value)}
              aria-label="Choisir un modèle de signature enregistré"
            >
              <option value="">
                {templates.length
                  ? "Choisir un modèle enregistré…"
                  : "Aucun modèle enregistré pour l'instant"}
              </option>
              {templates.map((template) => (
                <option key={template.id} value={template.id}>
                  {template.name}
                </option>
              ))}
            </select>
            <button
              className="button"
              disabled={!templateChoice}
              onClick={() => void deleteTemplate()}
              title="Supprimer le modèle sélectionné"
            >
              Supprimer
            </button>
          </div>
          <div className="signature-template-row">
            <input
              type="text"
              placeholder="Nom du nouveau modèle (ex. Directeur RH)"
              value={templateName}
              maxLength={80}
              onChange={(event) => setTemplateName(event.target.value)}
            />
            <button
              className="button"
              disabled={!signature || !templateName.trim()}
              onClick={() => void saveTemplate()}
            >
              Enregistrer
            </button>
          </div>
          {!isNative && (
            <small>
              Les modèles réutilisables sont conservés par l'application bureau.
            </small>
          )}
        </section>

        <SignatureStudio onUse={useSignature} notify={notify} />

        {signature && (
          <section className="panel signature-preview-panel">
            <h3>Signature active</h3>
            <div className="signature-checkerboard">
              <img src={signature.dataURL} alt="Signature active" />
            </div>
          </section>
        )}
      </aside>

      {printerDialog && printerState && (
        <PrinterDialog
          state={printerState}
          close={() => setPrinterDialog(false)}
          choose={(name) => void chooseDefaultPrinter(name)}
        />
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Aperçu de page avec signature déplaçable et redimensionnable.       */
/* ------------------------------------------------------------------ */

function PageCanvas({
  page,
  signature,
  placement,
  onPlacementChange,
  onPlacementRemove,
}: {
  page: HTMLCanvasElement;
  signature: ActiveSignature | null;
  placement: { x: number; y: number; width: number } | null;
  onPlacementChange: (next: { x: number; y: number; width: number }) => void;
  onPlacementRemove: () => void;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  const canvasHostRef = useRef<HTMLDivElement>(null);
  const dragRef = useRef<{
    mode: "move" | "resize";
    startX: number;
    startY: number;
    origin: { x: number; y: number; width: number };
  } | null>(null);

  useEffect(() => {
    const host = canvasHostRef.current;
    if (!host) return;
    host.innerHTML = "";
    page.className = "signature-page-canvas";
    host.appendChild(page);
  }, [page]);

  function fractionsFromEvent(
    event: ReactPointerEvent,
    origin: { x: number; y: number; width: number },
    start: { x: number; y: number },
    mode: "move" | "resize",
  ) {
    const bounds = hostRef.current?.getBoundingClientRect();
    if (!bounds) return origin;
    const deltaX = (event.clientX - start.x) / bounds.width;
    const deltaY = (event.clientY - start.y) / bounds.height;
    if (mode === "move") {
      return {
        x: Math.max(0, Math.min(0.98 - origin.width, origin.x + deltaX)),
        y: Math.max(0, Math.min(0.96, origin.y + deltaY)),
        width: origin.width,
      };
    }
    return {
      x: origin.x,
      y: origin.y,
      width: Math.max(0.06, Math.min(0.9, origin.width + deltaX)),
    };
  }

  function beginDrag(event: ReactPointerEvent, mode: "move" | "resize") {
    if (!placement) return;
    event.preventDefault();
    (event.target as Element).setPointerCapture(event.pointerId);
    dragRef.current = {
      mode,
      startX: event.clientX,
      startY: event.clientY,
      origin: { ...placement },
    };
  }

  function moveDrag(event: ReactPointerEvent) {
    const drag = dragRef.current;
    if (!drag) return;
    onPlacementChange(
      fractionsFromEvent(
        event,
        drag.origin,
        { x: drag.startX, y: drag.startY },
        drag.mode,
      ),
    );
  }

  const aspect = signature
    ? signature.pixelHeight / Math.max(1, signature.pixelWidth)
    : 0.4;

  return (
    <div className="signature-page-frame" ref={hostRef}>
      <div ref={canvasHostRef} className="signature-page-host" />
      {signature && placement && (
        <div
          className="signature-overlay"
          style={{
            left: `${placement.x * 100}%`,
            top: `${placement.y * 100}%`,
            width: `${placement.width * 100}%`,
            aspectRatio: `${1 / aspect}`,
          }}
          onPointerDown={(event) => beginDrag(event, "move")}
          onPointerMove={moveDrag}
          onPointerUp={() => (dragRef.current = null)}
          onPointerCancel={() => (dragRef.current = null)}
        >
          <img src={signature.dataURL} alt="Signature positionnée" draggable={false} />
          <button
            className="signature-overlay-remove"
            title="Retirer la signature de cette page"
            onPointerDown={(event) => event.stopPropagation()}
            onClick={onPlacementRemove}
          >
            ×
          </button>
          <span
            className="signature-overlay-resize"
            title="Redimensionner"
            onPointerDown={(event) => {
              event.stopPropagation();
              beginDrag(event, "resize");
            }}
            onPointerMove={moveDrag}
            onPointerUp={() => (dragRef.current = null)}
          />
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Studio : dessiner une signature ou nettoyer une photo/un scan.      */
/* ------------------------------------------------------------------ */

const INK_PREVIEW: Record<InkMode, string> = {
  noir: "rgb(24, 26, 34)",
  bleu: "rgb(23, 52, 134)",
  original: "rgb(90, 90, 100)",
};

function SignatureStudio({
  onUse,
  notify,
}: {
  onUse: (signature: ActiveSignature) => void;
  notify: (message: string) => void;
}) {
  const [tab, setTab] = useState<"dessiner" | "importer">("dessiner");
  return (
    <section className="panel signature-studio">
      <div className="attendance-tabs signature-studio-tabs" role="tablist">
        <button
          role="tab"
          aria-selected={tab === "dessiner"}
          className={tab === "dessiner" ? "active" : ""}
          onClick={() => setTab("dessiner")}
        >
          Dessiner
        </button>
        <button
          role="tab"
          aria-selected={tab === "importer"}
          className={tab === "importer" ? "active" : ""}
          onClick={() => setTab("importer")}
        >
          Photo / scan
        </button>
      </div>
      {tab === "dessiner" ? (
        <DrawPad onUse={onUse} />
      ) : (
        <ImportCleaner onUse={onUse} notify={notify} />
      )}
    </section>
  );
}

type Stroke = { points: { x: number; y: number; width: number }[]; color: string };

function DrawPad({ onUse }: { onUse: (signature: ActiveSignature) => void }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const strokesRef = useRef<Stroke[]>([]);
  const activeRef = useRef<Stroke | null>(null);
  const lastRef = useRef<{ x: number; y: number; time: number } | null>(null);
  const [ink, setInk] = useState<InkMode>("noir");
  const [hasInk, setHasInk] = useState(false);

  const repaint = useCallback(() => {
    const canvas = canvasRef.current;
    const context = canvas?.getContext("2d");
    if (!canvas || !context) return;
    context.clearRect(0, 0, canvas.width, canvas.height);
    for (const stroke of strokesRef.current) {
      drawStroke(context, stroke);
    }
    if (activeRef.current) drawStroke(context, activeRef.current);
  }, []);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ratio = Math.min(2, window.devicePixelRatio || 1);
    canvas.width = Math.round(canvas.clientWidth * ratio);
    canvas.height = Math.round(canvas.clientHeight * ratio);
    repaint();
  }, [repaint]);

  function pointFrom(event: ReactPointerEvent): { x: number; y: number } {
    const canvas = canvasRef.current!;
    const bounds = canvas.getBoundingClientRect();
    return {
      x: ((event.clientX - bounds.left) / bounds.width) * canvas.width,
      y: ((event.clientY - bounds.top) / bounds.height) * canvas.height,
    };
  }

  function strokeWidth(event: ReactPointerEvent, x: number, y: number): number {
    const canvas = canvasRef.current!;
    const base = canvas.width / 220;
    if (event.pressure && event.pointerType === "pen") {
      return base * (0.6 + event.pressure * 2);
    }
    const last = lastRef.current;
    const now = performance.now();
    if (!last) return base * 1.4;
    const distance = Math.hypot(x - last.x, y - last.y);
    const speed = distance / Math.max(1, now - last.time);
    // Trait plus fin quand la main va vite : rendu plume réaliste.
    return base * Math.max(0.6, Math.min(2.2, 2.2 - speed * 0.9));
  }

  function down(event: ReactPointerEvent) {
    event.preventDefault();
    (event.target as Element).setPointerCapture(event.pointerId);
    const { x, y } = pointFrom(event);
    lastRef.current = { x, y, time: performance.now() };
    activeRef.current = {
      color: INK_PREVIEW[ink],
      points: [{ x, y, width: strokeWidth(event, x, y) }],
    };
    repaint();
  }

  function move(event: ReactPointerEvent) {
    const active = activeRef.current;
    if (!active) return;
    const native = event.nativeEvent as globalThis.PointerEvent;
    const events =
      typeof native.getCoalescedEvents === "function"
        ? native.getCoalescedEvents()
        : [native];
    for (const sample of events) {
      const canvas = canvasRef.current!;
      const bounds = canvas.getBoundingClientRect();
      const x = ((sample.clientX - bounds.left) / bounds.width) * canvas.width;
      const y = ((sample.clientY - bounds.top) / bounds.height) * canvas.height;
      const previous = active.points[active.points.length - 1];
      const width = strokeWidth(event, x, y);
      // Largeur lissée pour éviter les à-coups.
      active.points.push({ x, y, width: previous.width * 0.7 + width * 0.3 });
      lastRef.current = { x, y, time: performance.now() };
    }
    repaint();
  }

  function up() {
    if (activeRef.current && activeRef.current.points.length > 1) {
      strokesRef.current.push(activeRef.current);
      setHasInk(true);
    }
    activeRef.current = null;
    lastRef.current = null;
    repaint();
  }

  function undo() {
    strokesRef.current.pop();
    setHasInk(strokesRef.current.length > 0);
    repaint();
  }

  function clear() {
    strokesRef.current = [];
    setHasInk(false);
    repaint();
  }

  function recolor(mode: InkMode) {
    setInk(mode);
    for (const stroke of strokesRef.current) stroke.color = INK_PREVIEW[mode];
    repaint();
  }

  function exportSignature() {
    const canvas = canvasRef.current;
    const context = canvas?.getContext("2d");
    if (!canvas || !context || strokesRef.current.length === 0) return;
    const image = context.getImageData(0, 0, canvas.width, canvas.height);
    let minX = canvas.width;
    let minY = canvas.height;
    let maxX = 0;
    let maxY = 0;
    for (let y = 0; y < canvas.height; y++) {
      for (let x = 0; x < canvas.width; x++) {
        if (image.data[(y * canvas.width + x) * 4 + 3] > 16) {
          if (x < minX) minX = x;
          if (x > maxX) maxX = x;
          if (y < minY) minY = y;
          if (y > maxY) maxY = y;
        }
      }
    }
    if (maxX <= minX || maxY <= minY) return;
    const padding = Math.round(canvas.width * 0.02);
    minX = Math.max(0, minX - padding);
    minY = Math.max(0, minY - padding);
    maxX = Math.min(canvas.width - 1, maxX + padding);
    maxY = Math.min(canvas.height - 1, maxY + padding);
    const output = window.document.createElement("canvas");
    output.width = maxX - minX + 1;
    output.height = maxY - minY + 1;
    output
      .getContext("2d")!
      .drawImage(
        canvas,
        minX,
        minY,
        output.width,
        output.height,
        0,
        0,
        output.width,
        output.height,
      );
    onUse({
      dataURL: output.toDataURL("image/png"),
      pixelWidth: output.width,
      pixelHeight: output.height,
    });
  }

  return (
    <div className="signature-drawpad">
      <canvas
        ref={canvasRef}
        className="signature-draw-canvas"
        onPointerDown={down}
        onPointerMove={move}
        onPointerUp={up}
        onPointerCancel={up}
      />
      <InkPicker value={ink} set={recolor} />
      <div className="signature-studio-actions">
        <button className="button" onClick={undo} disabled={!hasInk}>
          Annuler le trait
        </button>
        <button className="button" onClick={clear} disabled={!hasInk}>
          Effacer
        </button>
        <button
          className="button primary"
          onClick={exportSignature}
          disabled={!hasInk}
        >
          Utiliser cette signature
        </button>
      </div>
    </div>
  );
}

function drawStroke(context: CanvasRenderingContext2D, stroke: Stroke) {
  context.strokeStyle = stroke.color;
  context.fillStyle = stroke.color;
  context.lineCap = "round";
  context.lineJoin = "round";
  const points = stroke.points;
  if (points.length === 1) {
    context.beginPath();
    context.arc(points[0].x, points[0].y, points[0].width / 2, 0, Math.PI * 2);
    context.fill();
    return;
  }
  for (let index = 1; index < points.length; index++) {
    const from = points[index - 1];
    const to = points[index];
    context.beginPath();
    context.lineWidth = (from.width + to.width) / 2;
    const midX = (from.x + to.x) / 2;
    const midY = (from.y + to.y) / 2;
    context.moveTo(from.x, from.y);
    context.quadraticCurveTo(from.x, from.y, midX, midY);
    context.lineTo(to.x, to.y);
    context.stroke();
  }
}

/* ------------------------------------------------------------------ */
/* Import photo/scan : détourage IA optionnel puis nettoyage d'encre.  */
/* ------------------------------------------------------------------ */

function ImportCleaner({
  onUse,
  notify,
}: {
  onUse: (signature: ActiveSignature) => void;
  notify: (message: string) => void;
}) {
  const [source, setSource] = useState<RawImage | null>(null);
  const [options, setOptions] = useState<CleanupOptions>(defaultCleanupOptions);
  const [preview, setPreview] = useState<ActiveSignature | null>(null);
  const [busy, setBusy] = useState("");
  const [aiProgress, setAiProgress] = useState("");

  useEffect(() => {
    if (!source) return;
    const handle = window.setTimeout(() => {
      const result = cleanSignature(source, options);
      if (result.inkPixels === 0) {
        setPreview(null);
        return;
      }
      const canvas = window.document.createElement("canvas");
      canvas.width = result.image.width;
      canvas.height = result.image.height;
      const context = canvas.getContext("2d")!;
      context.putImageData(
        new ImageData(
          new Uint8ClampedArray(result.image.data),
          result.image.width,
          result.image.height,
        ),
        0,
        0,
      );
      setPreview({
        dataURL: canvas.toDataURL("image/png"),
        pixelWidth: canvas.width,
        pixelHeight: canvas.height,
      });
    }, 120);
    return () => window.clearTimeout(handle);
  }, [source, options]);

  async function readFile(file: File | null, withAI: boolean) {
    if (!file) return;
    setBusy("Lecture de l'image…");
    try {
      let image = await imageFromBlob(file);
      if (withAI) {
        image = await aiCutout(file, setAiProgress);
      }
      setSource(flattenToWhite(image));
      notify(
        withAI
          ? "Détourage IA terminé; affinez le nettoyage si besoin."
          : "Image chargée; le nettoyage automatique est appliqué.",
      );
    } catch (error) {
      notify(friendly(error));
    } finally {
      setBusy("");
      setAiProgress("");
    }
  }

  return (
    <div className="signature-import">
      <div className="signature-import-buttons">
        <FilePick
          label="Choisir une photo / un scan"
          onPick={(file) => void readFile(file, false)}
        />
        <FilePick
          label="Choisir + détourage IA"
          hint="Télécharge un modèle (~45 Mo) la première fois"
          onPick={(file) => void readFile(file, true)}
        />
      </div>
      {(busy || aiProgress) && (
        <p className="signature-working">{aiProgress || busy}</p>
      )}
      {source && (
        <>
          <InkPicker
            value={options.ink}
            set={(ink) => setOptions((current) => ({ ...current, ink }))}
          />
          <Slider
            label="Seuil d'encre"
            min={-50}
            max={50}
            value={options.thresholdBias}
            set={(thresholdBias) =>
              setOptions((current) => ({ ...current, thresholdBias }))
            }
          />
          <Slider
            label="Lissage"
            min={0}
            max={3}
            value={options.smoothing}
            set={(smoothing) => setOptions((current) => ({ ...current, smoothing }))}
          />
          <Slider
            label="Épaisseur"
            min={-2}
            max={2}
            value={options.thickness}
            set={(thickness) => setOptions((current) => ({ ...current, thickness }))}
          />
          <div className="signature-checkerboard">
            {preview ? (
              <img src={preview.dataURL} alt="Signature nettoyée" />
            ) : (
              <small>
                Aucune encre détectée : ajustez le seuil ou reprenez la photo.
              </small>
            )}
          </div>
          <div className="signature-studio-actions">
            <button
              className="button primary"
              disabled={!preview}
              onClick={() => preview && onUse(preview)}
            >
              Utiliser cette signature
            </button>
          </div>
        </>
      )}
    </div>
  );
}

function FilePick({
  label,
  hint,
  onPick,
}: {
  label: string;
  hint?: string;
  onPick: (file: File | null) => void;
}) {
  const id = useId();
  return (
    <label className="button signature-file-button" htmlFor={id} title={hint}>
      <input
        id={id}
        type="file"
        accept="image/*"
        onChange={(event) => {
          onPick(event.target.files?.[0] ?? null);
          event.target.value = "";
        }}
      />
      {label}
    </label>
  );
}

function InkPicker({ value, set }: { value: InkMode; set: (ink: InkMode) => void }) {
  const choices: { mode: InkMode; label: string }[] = [
    { mode: "noir", label: "Encre noire" },
    { mode: "bleu", label: "Encre bleue" },
    { mode: "original", label: "Couleur d'origine" },
  ];
  return (
    <div className="signature-ink-picker" role="radiogroup" aria-label="Encre">
      {choices.map((choice) => (
        <button
          key={choice.mode}
          role="radio"
          aria-checked={value === choice.mode}
          className={value === choice.mode ? "active" : ""}
          onClick={() => set(choice.mode)}
        >
          <i style={{ background: INK_PREVIEW[choice.mode] }} />
          {choice.label}
        </button>
      ))}
    </div>
  );
}

function Slider({
  label,
  min,
  max,
  value,
  set,
}: {
  label: string;
  min: number;
  max: number;
  value: number;
  set: (value: number) => void;
}) {
  const id = useId();
  return (
    <label className="signature-slider" htmlFor={id}>
      <span>
        {label}
        <b>{value}</b>
      </span>
      <input
        id={id}
        type="range"
        min={min}
        max={max}
        step={1}
        value={value}
        onChange={(event) => set(Number(event.target.value))}
      />
    </label>
  );
}

/* ------------------------------------------------------------------ */
/* Outils image partagés.                                              */
/* ------------------------------------------------------------------ */

async function imageFromBlob(blob: Blob): Promise<HTMLImageElement> {
  const url = URL.createObjectURL(blob);
  try {
    return await loadImageURL(url);
  } finally {
    URL.revokeObjectURL(url);
  }
}

function flattenToWhite(image: HTMLImageElement): RawImage {
  const scale = Math.min(
    1,
    maxWorkingPixels / Math.max(image.width, image.height),
  );
  const canvas = window.document.createElement("canvas");
  canvas.width = Math.max(1, Math.round(image.width * scale));
  canvas.height = Math.max(1, Math.round(image.height * scale));
  const context = canvas.getContext("2d", { willReadFrequently: true })!;
  context.fillStyle = "#ffffff";
  context.fillRect(0, 0, canvas.width, canvas.height);
  context.drawImage(image, 0, 0, canvas.width, canvas.height);
  const data = context.getImageData(0, 0, canvas.width, canvas.height);
  return { data: data.data, width: canvas.width, height: canvas.height };
}

async function aiCutout(
  file: File,
  onProgress: (message: string) => void,
): Promise<HTMLImageElement> {
  const { removeBackground, subscribeToProgress } = await import("rembg-webgpu");
  const unsubscribe = subscribeToProgress(({ phase, progress }) => {
    if (phase === "downloading")
      onProgress(`Téléchargement du modèle IA… ${Math.round(progress)}%`);
    else if (phase === "building") onProgress("Préparation du modèle IA…");
    else if (phase === "ready") onProgress("Détourage IA en cours…");
    else if (phase === "error") onProgress("");
  });
  const url = URL.createObjectURL(file);
  try {
    const result = await removeBackground(url);
    try {
      return await loadImageURL(result.blobUrl);
    } finally {
      URL.revokeObjectURL(result.blobUrl);
      if (result.previewUrl) URL.revokeObjectURL(result.previewUrl);
    }
  } catch (error) {
    throw new Error(
      `Détourage IA indisponible (${friendly(error)}). Utilisez le nettoyage automatique sans IA.`,
    );
  } finally {
    URL.revokeObjectURL(url);
    unsubscribe();
  }
}

/* ------------------------------------------------------------------ */
/* Choix de l'imprimante par défaut (premier usage et changements).    */
/* ------------------------------------------------------------------ */

function PrinterDialog({
  state,
  close,
  choose,
}: {
  state: PrinterState;
  close: () => void;
  choose: (name: string) => void;
}) {
  const initial =
    state.defaultPrinter ||
    state.printers.find((printer) => printer.systemDefault)?.name ||
    state.printers[0]?.name ||
    "";
  const [selection, setSelection] = useState(initial);
  return (
    <div className="backdrop" role="dialog" aria-modal="true">
      <div className="panel signature-printer-dialog">
        <h3>
          {state.configured
            ? "Changer d'imprimante par défaut"
            : "Choisissez votre imprimante"}
        </h3>
        <p>
          {state.configured
            ? "Les impressions de documents signés partiront vers cette imprimante."
            : "Premier usage : sélectionnez l'imprimante qui recevra vos documents signés. Vous pourrez en changer à tout moment."}
        </p>
        {state.printers.length === 0 ? (
          <p className="signature-working">
            Aucune imprimante détectée sur ce poste. Installez-en une puis
            rouvrez cette fenêtre.
          </p>
        ) : (
          <div className="signature-printer-list">
            {state.printers.map((printer) => (
              <label key={printer.name}>
                <input
                  type="radio"
                  name="imprimante"
                  checked={selection === printer.name}
                  onChange={() => setSelection(printer.name)}
                />
                <span>
                  {printer.name}
                  {printer.systemDefault && <small> · défaut système</small>}
                  {printer.storedDefault && <small> · choix actuel</small>}
                </span>
              </label>
            ))}
          </div>
        )}
        <div className="signature-studio-actions">
          <button className="button" onClick={close}>
            {state.configured ? "Annuler" : "Plus tard"}
          </button>
          <button
            className="button primary"
            disabled={!selection}
            onClick={() => choose(selection)}
          >
            Utiliser cette imprimante
          </button>
        </div>
      </div>
    </div>
  );
}

export type { SignatureTemplate, PrinterState };
