import type { Employee } from "./employee-validation";

export type CNSSNotice = {
  code: string;
  title: string;
  detail: string;
  employeeIDs?: string[];
};

export type CNSSReadiness = {
  canGenerate: boolean;
  entrantCount: number;
  errors: CNSSNotice[];
  warnings: CNSSNotice[];
};

export type CNSSErrorNotice = CNSSNotice & {
  cancelled: boolean;
};

const employeeList = (employees: Employee[]) =>
  employees.slice(0, 4).map((employee) => employee.id);

export function buildCNSSReadiness(
  employees: Employee[],
  periodStatus: string,
  totalEmployeeCount = employees.length,
): CNSSReadiness {
  const errors: CNSSNotice[] = [];
  const warnings: CNSSNotice[] = [];
  const entrants = employees.filter((employee) => !employee.cnss.trim());

  if (totalEmployeeCount === 0) {
    errors.push({
      code: "BDS-001",
      title: "Période sans salarié",
      detail:
        "Ajoutez ou importez les salariés de ce mois avant de créer le BDS.",
    });
  }

  const duplicates = new Map<string, Employee[]>();
  for (const employee of employees) {
    const cnss = employee.cnss.replace(/\D/g, "");
    if (!cnss) continue;
    duplicates.set(cnss, [...(duplicates.get(cnss) ?? []), employee]);
  }
  const duplicateEmployees = [...duplicates.values()]
    .filter((matches) => matches.length > 1)
    .flat();
  if (duplicateEmployees.length) {
    errors.push({
      code: "BDS-002",
      title: "Numéro CNSS utilisé plusieurs fois",
      detail:
        "Chaque numéro CNSS doit appartenir à un seul salarié dans la période.",
      employeeIDs: employeeList(duplicateEmployees),
    });
  }

  const invalidCNSS = employees.filter((employee) => {
    const value = employee.cnss.trim();
    return value !== "" && !/^\d{9}$/.test(value);
  });
  if (invalidCNSS.length) {
    errors.push({
      code: "BDS-003",
      title: "Numéro CNSS incomplet",
      detail:
        "Saisissez exactement 9 chiffres ou laissez le champ vide pour un entrant.",
      employeeIDs: employeeList(invalidCNSS),
    });
  }

  const invalidIdentity = employees.filter(
    (employee) =>
      !employee.firstName.trim() ||
      !employee.lastName.trim() ||
      !/^[A-Z]{1,2}\d{3,8}$/.test(employee.cin.replaceAll(" ", "")),
  );
  if (invalidIdentity.length) {
    errors.push({
      code: "BDS-004",
      title: "Identité incomplète",
      detail:
        "Renseignez le prénom, le nom et un CIN valide pour chaque salarié.",
      employeeIDs: employeeList(invalidIdentity),
    });
  }

  const invalidPayroll = employees.filter(
    (employee) =>
      employee.daysWorked < 1 ||
      employee.daysWorked > 26 ||
      employee.grossCentimes <= 0,
  );
  if (invalidPayroll.length) {
    errors.push({
      code: "BDS-005",
      title: "Valeurs mensuelles incompatibles",
      detail:
        "Le BDS normal exige 1 à 26 jours et un salaire brut supérieur à zéro.",
      employeeIDs: employeeList(invalidPayroll),
    });
  }

  if (entrants.length) {
    warnings.push({
      code: "BDS-A01",
      title: `${entrants.length} nouvel entrant${entrants.length > 1 ? "s" : ""} détecté${entrants.length > 1 ? "s" : ""}`,
      detail:
        "Sans numéro CNSS, ces salariés seront ajoutés en B04 avec le matricule 000000000.",
      employeeIDs: employeeList(entrants),
    });
  }
  if (totalEmployeeCount > employees.length) {
    warnings.push({
      code: "BDS-A00",
      title: `Aperçu borné à ${employees.length.toLocaleString("fr")} fiches`,
      detail: `La génération contrôlera les ${totalEmployeeCount.toLocaleString("fr")} salariés directement sur disque, sans les charger tous en mémoire.`,
    });
  }
  if (periodStatus !== "closed") {
    warnings.push({
      code: "BDS-A02",
      title: "Période encore modifiable",
      detail:
        "Vous pouvez générer un contrôle, mais régénérez le fichier après toute correction de paie.",
    });
  }

  return {
    canGenerate: errors.length === 0,
    entrantCount: entrants.length,
    errors,
    warnings,
  };
}

export function describeCNSSError(error: unknown): CNSSErrorNotice {
  const detail = String(error).replace(/^Error:\s*/, "");
  const normalized = detail.toLocaleLowerCase("fr");
  const notice = (code: string, title: string, guidance: string) => ({
    code,
    title,
    detail: `${guidance} Détail : ${detail}`,
    cancelled: false,
  });

  if (normalized.includes("annulée")) {
    return {
      code: "BDS-000",
      title: "Génération annulée",
      detail: "Aucun fichier n’a été créé ou remplacé.",
      cancelled: true,
    };
  }
  if (normalized.includes("période") && normalized.includes("préétabli"))
    return notice(
      "BDS-101",
      "Mois du préétabli incorrect",
      "Choisissez le préétabli CNSS correspondant exactement à la période active.",
    );
  if (normalized.includes("ne sont pas triés"))
    return notice(
      "BDS-102",
      "Préétabli non trié",
      "Téléchargez à nouveau le fichier officiel depuis Damancom sans le modifier.",
    );
  if (normalized.includes("absent de la paie"))
    return notice(
      "BDS-103",
      "Salarié du préétabli absent",
      "Ajoutez le salarié à la période ou traitez sa situation de sortie avant l’export.",
    );
  if (normalized.includes("plusieurs salariés"))
    return notice(
      "BDS-104",
      "Immatriculation CNSS dupliquée",
      "Corrigez les fiches indiquées pour conserver un seul salarié par numéro CNSS.",
    );
  if (normalized.includes("largeur cnss"))
    return notice(
      "BDS-105",
      "Valeur trop longue pour le format CNSS",
      "Corrigez la valeur signalée; le format e-BDS impose des largeurs fixes.",
    );
  if (
    normalized.includes("permission") ||
    normalized.includes("refusé") ||
    normalized.includes("création du bds")
  )
    return notice(
      "BDS-106",
      "Destination non accessible",
      "Choisissez un dossier dans lequel votre compte peut enregistrer un fichier.",
    );
  return notice(
    "BDS-199",
    "Le BDS n’a pas été généré",
    "Aucun fichier incomplet n’a été conservé. Corrigez le point signalé puis réessayez.",
  );
}
