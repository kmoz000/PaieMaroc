import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import App from "./App";
import { nextEmployeeID, validateEmployee } from "./employee-validation";

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

  it("accepts an empty CNSS number for an employee awaiting registration", () => {
    expect(validateEmployee({ ...validEmployee, cnss: "" })).toEqual([]);
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

describe("CNSS visual workflow", () => {
  it("shows the preflight, guided stages and warnings before export", async () => {
    const user = userEvent.setup();
    render(<App />);

    const cnssNavigation = await screen.findByRole(
      "button",
      { name: /Déclaration CNSS/ },
      { timeout: 2000 },
    );
    await user.click(cnssNavigation);

    expect(screen.getByTestId("cnss-workbench")).toBeVisible();
    expect(screen.getByText("Contrôler avant de remettre")).toBeVisible();
    expect(
      screen.getByRole("list", { name: "Étapes de génération BDS" }),
    ).toBeVisible();
    expect(screen.getByText("Période encore modifiable")).toBeVisible();
    expect(
      screen.getByRole("button", { name: /Choisir et générer/ }),
    ).toBeEnabled();
  });
});

describe("attendance journal", () => {
  it("guides the user from raw punches to calendar and work rules", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(
      await screen.findByRole(
        "button",
        { name: /Journal de pointage/ },
        { timeout: 2000 },
      ),
    );

    expect(screen.getByRole("heading", { name: "Journal de pointage" })).toBeVisible();
    expect(screen.getAllByText("Lina Benjelloun")[0]).toBeVisible();
    expect(screen.getByText(/événement\(s\) attendent un salarié/)).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Calendrier & coefficients" }));
    expect(screen.getByRole("heading", { name: /Calendrier/ })).toBeVisible();
    expect(screen.getByRole("heading", { name: "Ajouter un jour spécial" })).toBeVisible();

    await user.click(screen.getByRole("button", { name: "Règles de travail" }));
    expect(screen.getByRole("heading", { name: "Temps de travail et majorations" })).toBeVisible();
    expect(screen.getByText("Garde-fou de calcul")).toBeVisible();
  });
});

describe("pointer connectivity", () => {
  it("separates local polling, ADMS firewall consent and userspace WireGuard", async () => {
    const user = userEvent.setup();
    render(<App />);

    await user.click(
      await screen.findByRole(
        "button",
        { name: /Pointeuses ZKTeco/ },
        { timeout: 2000 },
      ),
    );

    expect(screen.getByTestId("pointer-connectivity")).toBeVisible();
    expect(screen.getByText(/Aucun port entrant à ouvrir/)).toBeVisible();
    expect(screen.getByText(/Aucune route système ni interface noyau/)).toBeVisible();

    await user.click(screen.getByRole("button", { name: /Configurer/ }));
    await user.click(
      screen.getByRole("checkbox", {
        name: /Recevoir les journaux des pointeuses/,
      }),
    );
    expect(screen.getByText("Autorisation du pare-feu")).toBeVisible();
    expect(
      screen.getByRole("checkbox", {
        name: /J’autorise cette écoute entrante/,
      }),
    ).toBeRequired();

    await user.click(
      screen.getByRole("checkbox", {
        name: /Activer le tunnel privé des pointeuses/,
      }),
    );
    expect(screen.getByLabelText(/Passerelle WireGuard/)).toBeVisible();
    expect(screen.getByText(/Les routes par défaut sont refusées/)).toBeVisible();
  });
});
