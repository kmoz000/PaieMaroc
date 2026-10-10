import { describe, expect, it } from "vitest";
import type { Employee } from "./employee-validation";
import { buildCNSSReadiness, describeCNSSError } from "./cnss-guidance";

const employee = (overrides: Partial<Employee> = {}): Employee => ({
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
  ...overrides,
});

describe("CNSS readiness", () => {
  it("blocks an empty period with an actionable code", () => {
    const result = buildCNSSReadiness([], "open");

    expect(result.canGenerate).toBe(false);
    expect(result.errors[0].code).toBe("BDS-001");
  });

  it("detects duplicate CNSS numbers before opening a file", () => {
    const result = buildCNSSReadiness(
      [employee(), employee({ id: "EMP-0002" })],
      "closed",
    );

    expect(result.canGenerate).toBe(false);
    expect(result.errors).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          code: "BDS-002",
          employeeIDs: ["EMP-0001", "EMP-0002"],
        }),
      ]),
    );
  });

  it("warns that a missing CNSS number creates a B04 entrant", () => {
    const result = buildCNSSReadiness([employee({ cnss: "" })], "closed");

    expect(result.canGenerate).toBe(true);
    expect(result.entrantCount).toBe(1);
    expect(result.warnings[0]).toMatchObject({ code: "BDS-A01" });
  });

  it("keeps large periods bounded and announces the complete disk control", () => {
    const result = buildCNSSReadiness([employee()], "closed", 1_000_000);

    expect(result.canGenerate).toBe(true);
    expect(result.warnings).toEqual(
      expect.arrayContaining([expect.objectContaining({ code: "BDS-A00" })]),
    );
  });
});

describe("CNSS error guidance", () => {
  it("explains a pre-established period mismatch", () => {
    expect(
      describeCNSSError(
        new Error("la période du préétabli 202609 ne correspond pas à 202610"),
      ),
    ).toMatchObject({ code: "BDS-101", cancelled: false });
  });

  it("treats file picker cancellation as non-destructive information", () => {
    expect(describeCNSSError(new Error("génération CNSS annulée"))).toEqual({
      code: "BDS-000",
      title: "Génération annulée",
      detail: "Aucun fichier n’a été créé ou remplacé.",
      cancelled: true,
    });
  });
});
