import {render, screen} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {useState} from "react";
import {describe, expect, it} from "vitest";
import {DateField, MonthField, formatDateInput, parseDateInput, parseMonthInput} from "./date-picker";

describe("PaieMaroc date picker", () => {
  it("parses fast Moroccan date input without timezone conversion", () => {
    expect(parseDateInput("10/10/2026")).toBe("2026-10-10");
    expect(parseDateInput("10102026")).toBe("2026-10-10");
    expect(parseDateInput("2026-10-10")).toBe("2026-10-10");
    expect(formatDateInput("2026-10-10")).toBe("10/10/2026");
    expect(parseDateInput("31/02/2026")).toBeNull();
    expect(parseMonthInput("102026")).toBe("2026-10");
    expect(parseMonthInput("13/2026")).toBeNull();
  });

  it("supports keyboard entry and reports invalid calendar dates", async () => {
    const user = userEvent.setup();
    function Fixture() {
      const [value, setValue] = useState("");
      return <><DateField label="Date" value={value} set={setValue} /><output>{value}</output></>;
    }
    render(<Fixture />);
    const input = screen.getByRole("textbox", {name: /Date/});
    await user.type(input, "10102026");
    expect(input).toHaveValue("10/10/2026");
    expect(screen.getByText("2026-10-10")).toBeVisible();

    await user.clear(input);
    await user.type(input, "31022026");
    expect(screen.getByText("Date invalide ou hors période autorisée.")).toBeVisible();
  });

  it("opens the inline calendar and selects a date", async () => {
    const user = userEvent.setup();
    function Fixture() {
      const [value, setValue] = useState("2026-10-10");
      return <><DateField label="Jour spécial" value={value} set={setValue} /><output>{value}</output></>;
    }
    render(<Fixture />);
    await user.click(screen.getByRole("button", {name: "Ouvrir le calendrier — Jour spécial"}));
    expect(screen.getByRole("dialog", {name: "Calendrier — Jour spécial"})).toBeVisible();
    await user.click(screen.getByRole("button", {name: /15 octobre 2026/}));
    expect(screen.getByText("2026-10-15")).toBeVisible();
    expect(screen.queryByRole("dialog", {name: "Calendrier — Jour spécial"})).toBeNull();
  });

  it("uses the custom month grid for payroll periods", async () => {
    const user = userEvent.setup();
    function Fixture() {
      const [value, setValue] = useState("2026-10");
      return <><MonthField label="Période" value={value} set={setValue} /><output>{value}</output></>;
    }
    render(<Fixture />);
    await user.click(screen.getByRole("button", {name: "Choisir le mois — Période"}));
    await user.click(screen.getByRole("button", {name: "nov."}));
    expect(screen.getByText("2026-11")).toBeVisible();
  });
});
