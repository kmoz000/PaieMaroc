import {describe, expect, it} from "vitest";
import {installInspectionGuard, isInspectionShortcut, type InspectionKey} from "./inspection-guard";

function shortcut(overrides: Partial<InspectionKey>): InspectionKey {
  return {
    altKey: false,
    ctrlKey: false,
    key: "",
    metaKey: false,
    shiftKey: false,
    ...overrides,
  };
}

describe("inspection guard", () => {
  it.each([
    shortcut({key: "F12"}),
    shortcut({ctrlKey: true, shiftKey: true, key: "I"}),
    shortcut({ctrlKey: true, shiftKey: true, key: "c"}),
    shortcut({metaKey: true, altKey: true, key: "J"}),
  ])("blocks inspection shortcuts", (event) => {
    expect(isInspectionShortcut(event)).toBe(true);
  });

  it.each([
    shortcut({ctrlKey: true, key: "C"}),
    shortcut({metaKey: true, key: "I"}),
    shortcut({key: "F11"}),
  ])("keeps ordinary application shortcuts", (event) => {
    expect(isInspectionShortcut(event)).toBe(false);
  });

  it("prevents the production context menu and inspector key", () => {
    const cleanup = installInspectionGuard();
    const menu = new MouseEvent("contextmenu", {bubbles: true, cancelable: true});
    const shortcutEvent = new KeyboardEvent("keydown", {key: "F12", bubbles: true, cancelable: true});
    document.dispatchEvent(menu);
    window.dispatchEvent(shortcutEvent);
    expect(menu.defaultPrevented).toBe(true);
    expect(shortcutEvent.defaultPrevented).toBe(true);
    cleanup();
  });
});
