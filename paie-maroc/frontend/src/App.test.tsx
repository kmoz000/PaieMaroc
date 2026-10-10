import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import App, { nextEmployeeID, validateEmployee } from "./App";

const validEmployee = {
  id: "EMP-0001",
  firstName: "Nadia",
  lastName: "Idrissi",
  cin: "AB123456",
  cnss: "123456789",
  rib: "011780000012345678901234",
  daysWorked: 26,
  dependants: 1,
  grossCentimes: 800_000,
  exemptCentimes: 0,
  advanceCentimes: 0,
};

describe("employee validation", () => {
  it("accepts a complete Moroccan payroll record", () => {
    expect(validateEmployee(validEmployee)).toEqual([]);
  });

  it("returns actionable errors before persistence", () => {
    const issues = validateEmployee({
      ...validEmployee,
      cnss: "12",
      rib: "123",
      daysWorked: 31,
    });
    expect(issues.map((issue) => issue.field)).toEqual([
      "cnss",
      "rib",
      "daysWorked",
    ]);
  });

  it("chooses the first available stable matricule", () => {
    expect(
      nextEmployeeID([
        { ...validEmployee, id: "EMP-0001" },
        { ...validEmployee, id: "EMP-0003" },
      ]),
    ).toBe("EMP-0002");
  });
});

describe("employee dialog", () => {
  it("shows validation feedback inside the open dialog", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(
      await screen.findByRole("button", { name: "Ajouter" }, { timeout: 2000 }),
    );
    await user.click(screen.getByRole("button", { name: "Enregistrer" }));

    expect(screen.getByRole("alert")).toHaveTextContent("champs signalés");
    expect(screen.getByText("Le prénom est obligatoire.")).toBeVisible();
    expect(screen.getByLabelText(/Prénom/)).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(screen.getByRole("dialog", { hidden: true })).toBeInTheDocument();
  });

  it("keeps a newly saved employee visible in the browser preview", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(
      await screen.findByRole("button", { name: "Ajouter" }, { timeout: 2000 }),
    );
    await user.type(screen.getByLabelText(/Prénom/), validEmployee.firstName);
    await user.type(screen.getByLabelText(/^Nom/), validEmployee.lastName);
    await user.type(screen.getByLabelText(/^CIN/), validEmployee.cin);
    await user.type(screen.getByLabelText(/^N° CNSS/), validEmployee.cnss);
    await user.type(screen.getByLabelText(/^RIB marocain/), validEmployee.rib);
    await user.click(screen.getByRole("button", { name: "Enregistrer" }));

    expect(await screen.findByText("Nadia Idrissi")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Enregistrer" })).toBeNull();
  });
});
