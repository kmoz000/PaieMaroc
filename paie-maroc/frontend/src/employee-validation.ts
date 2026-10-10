export type Employee = {
  id: string;
  firstName: string;
  lastName: string;
  cin: string;
  cnss: string;
  rib: string;
  daysWorked: number;
  dependants: number;
  grossCentimes: number;
  exemptCentimes: number;
  advanceCentimes: number;
};

export type EmployeeIssue = {
  field: keyof Employee;
  message: string;
};

export const employeePatterns = {
  id: "[A-Za-z0-9][A-Za-z0-9._/-]{0,31}",
  cin: "[A-Z]{1,2}[0-9]{3,8}",
  cnss: "(?:[0-9]{9})?",
  rib: "[0-9]{24}",
} as const;

export function validateEmployee(employee: Employee): EmployeeIssue[] {
  const issues: EmployeeIssue[] = [];
  const add = (field: keyof Employee, message: string) =>
    issues.push({ field, message });
  const digits = (value: string) => value.replace(/[^0-9]/g, "");

  if (!new RegExp(`^${employeePatterns.id}$`).test(employee.id.trim()))
    add(
      "id",
      "Utilisez 1 à 32 caractères : lettres, chiffres, point, tiret, barre ou soulignement.",
    );
  if (!employee.firstName.trim())
    add("firstName", "Le prénom est obligatoire.");
  if (!employee.lastName.trim()) add("lastName", "Le nom est obligatoire.");
  if (
    !new RegExp(`^${employeePatterns.cin}$`).test(
      employee.cin.replaceAll(" ", ""),
    )
  )
    add("cin", "Le CIN doit contenir 1 à 2 lettres puis 3 à 8 chiffres.");
  if (employee.cnss !== "" && digits(employee.cnss).length !== 9)
    add(
      "cnss",
      "Le numéro CNSS doit contenir exactement 9 chiffres, ou rester vide pour un nouvel entrant.",
    );
  if (digits(employee.rib).length !== 24)
    add("rib", "Le RIB marocain doit contenir exactement 24 chiffres.");
  if (employee.daysWorked < 0 || employee.daysWorked > 30)
    add(
      "daysWorked",
      "Les jours travaillés doivent être compris entre 0 et 30.",
    );
  if (employee.dependants < 0 || employee.dependants > 6)
    add(
      "dependants",
      "Les personnes à charge doivent être comprises entre 0 et 6.",
    );
  if (
    !Number.isSafeInteger(employee.grossCentimes) ||
    employee.grossCentimes <= 0
  )
    add("grossCentimes", "Le salaire brut doit être supérieur à zéro.");
  if (
    !Number.isSafeInteger(employee.exemptCentimes) ||
    employee.exemptCentimes < 0
  )
    add("exemptCentimes", "Le montant exonéré ne peut pas être négatif.");
  if (
    !Number.isSafeInteger(employee.advanceCentimes) ||
    employee.advanceCentimes < 0
  )
    add("advanceCentimes", "L’avance ne peut pas être négative.");
  return issues;
}

export function nextEmployeeID(employees: Employee[]): string {
  const identifiers = new Set(employees.map((employee) => employee.id));
  let sequence = 1;
  while (identifiers.has(`EMP-${String(sequence).padStart(4, "0")}`))
    sequence++;
  return `EMP-${String(sequence).padStart(4, "0")}`;
}
