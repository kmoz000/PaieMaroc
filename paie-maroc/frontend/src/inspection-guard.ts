export type InspectionKey = Pick<
  KeyboardEvent,
  "altKey" | "ctrlKey" | "key" | "metaKey" | "shiftKey"
>;

export function isInspectionShortcut(event: InspectionKey): boolean {
  const key = event.key.toUpperCase();
  return (
    key === "F12" ||
    ((event.ctrlKey || event.metaKey) &&
      event.shiftKey &&
      (key === "I" || key === "J" || key === "C")) ||
    (event.metaKey &&
      event.altKey &&
      (key === "I" || key === "J" || key === "C"))
  );
}

export function installInspectionGuard(): () => void {
  const blockContextMenu = (event: MouseEvent) => event.preventDefault();
  const blockShortcut = (event: KeyboardEvent) => {
    if (isInspectionShortcut(event)) {
      event.preventDefault();
      event.stopPropagation();
    }
  };
  document.addEventListener("contextmenu", blockContextMenu);
  window.addEventListener("keydown", blockShortcut);
  return () => {
    document.removeEventListener("contextmenu", blockContextMenu);
    window.removeEventListener("keydown", blockShortcut);
  };
}
