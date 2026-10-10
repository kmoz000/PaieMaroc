import { useEffect, useId, useMemo, useState } from "react";
import type { FormEvent, ReactElement, ReactNode } from "react";
import {
  AttendanceProfiles,
  CalculatePayroll,
  ClosePeriod,
  CreateInitialAdmin,
  CreatePeriod,
  CreateUser,
  DeletePeriod,
  GenerateCNSSBDS,
  GetAuthState,
  ImportEmployeesCSVToPeriod,
  LinkAttendanceEmployee,
  ListAttendanceDevices,
  ListPeriodEmployees,
  ListPeriods,
  ListUsers,
  Login,
  Logout,
  PreviewExport,
  ReopenPeriod,
  ResetUserPassword,
  SaveAttendanceDevice,
  SaveExport,
  SavePeriodEmployee,
  SetUserEnabled,
  TestAttendanceDevice,
  ValidatePayroll,
} from "../wailsjs/go/main/App";
import {
  Quit,
  WindowMinimise,
  WindowToggleMaximise,
} from "../wailsjs/runtime/runtime";
import brandLogo from "./assets/paie-maroc-hr-logo.png";
import whiteBrandLogo from "./assets/paie-maroc-hr-logo-white.png";
import "./App.css";

type User = {
  id: number;
  username: string;
  displayName: string;
  role: string;
  enabled: boolean;
  createdAt: string;
  lastLoginAt: string;
};
type AuthState = {
  setupRequired: boolean;
  authenticated: boolean;
  user?: User;
};
type Employee = {
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
type EmployeeIssue = {
  field: keyof Employee;
  message: string;
};
type EmployeeErrors = Partial<Record<keyof Employee, string>>;
type Row = {
  employee: Employee;
  employeeCnss: number;
  employeeAmo: number;
  incomeTax: number;
  netPay: number;
  employerCost: number;
};
type Batch = {
  rows: Row[];
  grossTotalCentimes: number;
  netTotalCentimes: number;
  employeeSocialTotal: number;
  incomeTaxTotal: number;
  employerChargesTotal: number;
  employerCostCentimes: number;
  rulesVersion: string;
};
type Period = {
  period: string;
  status: string;
  copiedFrom: string;
  employeeCount: number;
  grossCentimes: number;
  createdAt: string;
  closedAt: string;
};
type Device = {
  id: number;
  name: string;
  model: string;
  protocol: string;
  host: string;
  port: number;
  enabled: boolean;
  lastStatus: string;
  lastCheckedAt: string;
  linkedUsers: number;
};
type Route =
  | "overview"
  | "employees"
  | "payroll"
  | "cnss"
  | "exports"
  | "periods"
  | "devices"
  | "access";

const isNative = Boolean((window as unknown as { go?: unknown }).go);
const isMac = /Mac/i.test(navigator.platform);
const employeePatterns = {
  id: "[A-Za-z0-9][A-Za-z0-9._/-]{0,31}",
  cin: "[A-Z]{1,2}[0-9]{3,8}",
  cnss: "[0-9]{6,12}",
  rib: "[0-9]{24}",
} as const;
const demoEmployees: Employee[] = [
  {
    id: "AT-0018",
    firstName: "Lina",
    lastName: "Benjelloun",
    cin: "BE152487",
    cnss: "168764721",
    rib: "011780000012345678901234",
    daysWorked: 26,
    dependants: 2,
    grossCentimes: 1280000,
    exemptCentimes: 50000,
    advanceCentimes: 0,
  },
  {
    id: "AT-0024",
    firstName: "Yassine",
    lastName: "El Mansouri",
    cin: "BK652143",
    cnss: "123456789",
    rib: "022780000098765432109876",
    daysWorked: 26,
    dependants: 1,
    grossCentimes: 940000,
    exemptCentimes: 0,
    advanceCentimes: 100000,
  },
  {
    id: "AT-0031",
    firstName: "Salma",
    lastName: "Alaoui",
    cin: "AB782145",
    cnss: "987654321",
    rib: "007780000055555555555555",
    daysWorked: 24,
    dependants: 0,
    grossCentimes: 760000,
    exemptCentimes: 0,
    advanceCentimes: 0,
  },
];

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
  if (digits(employee.cnss).length < 6 || digits(employee.cnss).length > 12)
    add("cnss", "Le numéro CNSS doit contenir 6 à 12 chiffres.");
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

function employeeFieldFromError(message: string): keyof Employee | undefined {
  const field = message.match(
    /^(id|firstName|lastName|cin|cnss|rib|daysWorked|dependants|grossCentimes|exemptCentimes|advanceCentimes):/,
  )?.[1];
  return field as keyof Employee | undefined;
}
const demoUser: User = {
  id: 1,
  username: "admin.paie",
  displayName: "Karim Benali",
  role: "admin",
  enabled: true,
  createdAt: "",
  lastLoginAt: "",
};
const blankBatch: Batch = {
  rows: [],
  grossTotalCentimes: 0,
  netTotalCentimes: 0,
  employeeSocialTotal: 0,
  incomeTaxTotal: 0,
  employerChargesTotal: 0,
  employerCostCentimes: 0,
  rulesVersion: "MA-2026.1",
};
const blankDevice: Device = {
  id: 0,
  name: "Entrée principale",
  model: "MB10-VL",
  protocol: "standalone",
  host: "192.168.1.201",
  port: 4370,
  enabled: true,
  lastStatus: "never",
  lastCheckedAt: "",
  linkedUsers: 0,
};
const money = (value: number) =>
  new Intl.NumberFormat("fr-MA", {
    style: "currency",
    currency: "MAD",
    minimumFractionDigits: 2,
  }).format(value / 100);
const friendlyError = (error: unknown) =>
  String(error).replace(/^Error:\s*/, "");
const initials = (name: string) =>
  name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0])
    .join("")
    .toUpperCase();
const roleName = (role: string) =>
  role === "admin" ? "Administrateur" : "Gestionnaire paie";

function Icon({ name, size = 18 }: { name: string; size?: number }) {
  const p: Record<string, ReactElement> = {
    grid: (
      <>
        <rect x="3" y="3" width="7" height="7" rx="2" />
        <rect x="14" y="3" width="7" height="7" rx="2" />
        <rect x="3" y="14" width="7" height="7" rx="2" />
        <rect x="14" y="14" width="7" height="7" rx="2" />
      </>
    ),
    users: (
      <>
        <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
        <circle cx="9" cy="7" r="4" />
        <path d="M22 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8" />
      </>
    ),
    calc: (
      <>
        <rect x="4" y="2" width="16" height="20" rx="2" />
        <path d="M8 6h8M8 10h.01M12 10h.01M16 10h.01M8 14h.01M12 14h.01M16 14h.01M8 18h.01M12 18h.01M16 18h.01" />
      </>
    ),
    shield: (
      <>
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10" />
        <path d="m9 12 2 2 4-4" />
      </>
    ),
    check: <path d="M20 6 9 17l-5-5" />,
    send: (
      <>
        <path d="m22 2-7 20-4-9-9-4Z" />
        <path d="M22 2 11 13" />
      </>
    ),
    plus: <path d="M12 5v14M5 12h14" />,
    search: (
      <>
        <circle cx="11" cy="11" r="8" />
        <path d="m21 21-4.3-4.3" />
      </>
    ),
    lock: (
      <>
        <rect x="3" y="11" width="18" height="10" rx="2" />
        <path d="M7 11V7a5 5 0 0 1 10 0v4" />
      </>
    ),
    arrow: (
      <>
        <path d="M5 12h14M13 6l6 6-6 6" />
      </>
    ),
    database: (
      <>
        <ellipse cx="12" cy="5" rx="9" ry="3" />
        <path d="M3 5v14c0 1.7 4 3 9 3s9-1.3 9-3V5M3 12c0 1.7 4 3 9 3s9-1.3 9-3" />
      </>
    ),
    calendar: (
      <>
        <rect x="3" y="5" width="18" height="16" rx="2" />
        <path d="M16 3v4M8 3v4M3 11h18" />
      </>
    ),
    clock: (
      <>
        <circle cx="12" cy="12" r="9" />
        <path d="M12 7v5l3 2" />
      </>
    ),
    device: (
      <>
        <rect x="5" y="2" width="14" height="20" rx="2" />
        <circle cx="12" cy="17" r="2" />
        <path d="M9 6h6" />
      </>
    ),
    logout: (
      <>
        <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M16 17l5-5-5-5M21 12H9" />
      </>
    ),
    key: (
      <>
        <circle cx="8" cy="15" r="4" />
        <path d="m11 12 9-9M18 5l2 2M15 8l2 2" />
      </>
    ),
    warning: (
      <>
        <path d="M12 3 2.7 20h18.6Z" />
        <path d="M12 9v4M12 17h.01" />
      </>
    ),
    trash: (
      <>
        <path d="M3 6h18M8 6V4h8v2M19 6l-1 15H6L5 6M10 11v6M14 11v6" />
      </>
    ),
    more: (
      <>
        <circle cx="5" cy="12" r="1" />
        <circle cx="12" cy="12" r="1" />
        <circle cx="19" cy="12" r="1" />
      </>
    ),
  };
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {p[name]}
    </svg>
  );
}

function localCalc(employees: Employee[]): Batch {
  const pct = (v: number, r: number) => Math.round(v * r);
  const rows = employees.map((employee) => {
    const taxable = employee.grossCentimes - employee.exemptCentimes,
      employeeCnss = pct(Math.min(taxable, 600000), 0.0448),
      employeeAmo = pct(taxable, 0.0226),
      netPay =
        employee.grossCentimes -
        employeeCnss -
        employeeAmo -
        employee.advanceCentimes,
      charges = pct(Math.min(taxable, 600000), 0.0898) + pct(taxable, 0.1211);
    return {
      employee,
      employeeCnss,
      employeeAmo,
      incomeTax: 0,
      netPay,
      employerCost: employee.grossCentimes + charges,
    };
  });
  return {
    ...blankBatch,
    rows,
    grossTotalCentimes: employees.reduce((s, e) => s + e.grossCentimes, 0),
    netTotalCentimes: rows.reduce((s, r) => s + r.netPay, 0),
    employerChargesTotal: rows.reduce(
      (s, r) => s + r.employerCost - r.employee.grossCentimes,
      0,
    ),
  };
}

export default function App() {
  const [state, setState] = useState<AuthState | null>(null),
    [startupError, setStartupError] = useState(""),
    [bootReady, setBootReady] = useState(false);
  useEffect(() => {
    const timer = window.setTimeout(() => setBootReady(true), 720);
    return () => window.clearTimeout(timer);
  }, []);
  useEffect(() => {
    if (!isNative) {
      setState({ setupRequired: false, authenticated: true, user: demoUser });
      return;
    }
    GetAuthState()
      .then((value) => setState(value as AuthState))
      .catch((error) => setStartupError(friendlyError(error)));
  }, []);
  if (startupError)
    return (
      <DesktopFrame>
        <CenteredError message={startupError} />
      </DesktopFrame>
    );
  if (!state || !bootReady)
    return (
      <DesktopFrame>
        <BootSequence />
      </DesktopFrame>
    );
  if (state.setupRequired)
    return (
      <DesktopFrame>
        <AuthScreen mode="setup" onSuccess={setState} />
      </DesktopFrame>
    );
  if (!state.authenticated || !state.user)
    return (
      <DesktopFrame>
        <AuthScreen mode="login" onSuccess={setState} />
      </DesktopFrame>
    );
  return (
    <DesktopFrame>
      <Workspace
        user={state.user}
        onLogout={async () => {
          if (isNative) await Logout();
          setState({ setupRequired: false, authenticated: false });
        }}
      />
    </DesktopFrame>
  );
}

function DesktopFrame({ children }: { children: ReactNode }) {
  const toggleMaximise = () => {
    if (isNative && !isMac) WindowToggleMaximise();
  };
  return (
    <div className={`desktop-window ${isMac ? "platform-mac" : ""}`}>
      <header className="desktop-titlebar" onDoubleClick={toggleMaximise}>
        <div className="frame-origin">
          <strong>PaieMaroc</strong>
          <span>POSTE LOCAL</span>
        </div>
        <div className="frame-center">
          <Icon name="lock" size={11} />
          Espace de paie sécurisé
        </div>
        {isNative && !isMac && (
          <div className="window-controls">
            <button
              type="button"
              onClick={() => WindowMinimise()}
              aria-label="Réduire la fenêtre"
            >
              <i className="minimise" />
            </button>
            <button
              type="button"
              onClick={() => WindowToggleMaximise()}
              aria-label="Agrandir ou restaurer la fenêtre"
            >
              <i className="maximise" />
            </button>
            <button
              type="button"
              className="close-window"
              onClick={() => Quit()}
              aria-label="Fermer PaieMaroc"
            >
              <i className="close" />
            </button>
          </div>
        )}
      </header>
      <div className="desktop-body">{children}</div>
    </div>
  );
}

function BootSequence() {
  return (
    <div className="boot">
      <div className="boot-emblem">
        <img
          src={brandLogo}
          alt="PaieMaroc — Gestion des Ressources Humaines"
        />
      </div>
      <div className="boot-progress" aria-label="Chargement de PaieMaroc">
        <i />
      </div>
      <div className="boot-stages">
        <span>Base locale</span>
        <span>Règles de paie</span>
        <span>Espace prêt</span>
      </div>
      <p>Préparation de votre poste de paie…</p>
    </div>
  );
}

function AuthScreen({
  mode,
  onSuccess,
}: {
  mode: "setup" | "login";
  onSuccess: (state: AuthState) => void;
}) {
  const [displayName, setDisplayName] = useState(""),
    [username, setUsername] = useState(""),
    [password, setPassword] = useState(""),
    [confirmation, setConfirmation] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    if (mode === "setup" && password !== confirmation) {
      setError("Les deux mots de passe ne correspondent pas.");
      return;
    }
    setBusy(true);
    try {
      const next =
        mode === "setup"
          ? await CreateInitialAdmin({
              displayName,
              username,
              password,
              role: "admin",
            } as never)
          : await Login(username, password);
      onSuccess(next as AuthState);
    } catch (reason) {
      setError(friendlyError(reason));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth-shell">
      <section className="auth-story">
        <Brand light />
        <div className="story-copy">
          <span className="eyebrow">PAIE MAROCAINE, SANS FRICTION</span>
          <h1>
            {mode === "setup"
              ? "Créez votre espace de paie sécurisé."
              : "Heureux de vous revoir."}
          </h1>
          <p>
            Vos salariés, périodes, déclarations CNSS, remises bancaires et
            pointages réunis dans un parcours clair.
          </p>
          <ul>
            <li>
              <Icon name="check" />
              Données conservées sur cet appareil
            </li>
            <li>
              <Icon name="check" />
              Mots de passe protégés par Argon2id
            </li>
            <li>
              <Icon name="check" />
              Droits administrateur et gestionnaire séparés
            </li>
          </ul>
        </div>
        <small>PaieMaroc · logiciel libre pour les employeurs marocains</small>
      </section>
      <main className="auth-main">
        <form className="auth-card" onSubmit={submit}>
          <div className="auth-step">
            {mode === "setup" ? "1 sur 1" : "ACCÈS SÉCURISÉ"}
          </div>
          <h2>
            {mode === "setup" ? "Administrateur initial" : "Se connecter"}
          </h2>
          <p>
            {mode === "setup"
              ? "Ce premier compte pourra inviter et gérer les autres utilisateurs."
              : "Utilisez le compte créé par votre administrateur."}
          </p>
          {mode === "setup" && (
            <TextField
              label="Nom complet"
              value={displayName}
              set={setDisplayName}
              placeholder="Karim Benali"
              autoComplete="name"
            />
          )}
          <TextField
            label="Identifiant"
            value={username}
            set={setUsername}
            placeholder="admin.paie"
            autoComplete="username"
          />
          <TextField
            label="Mot de passe"
            value={password}
            set={setPassword}
            placeholder="12 caractères minimum"
            type="password"
            autoComplete={
              mode === "setup" ? "new-password" : "current-password"
            }
          />
          {mode === "setup" && (
            <TextField
              label="Confirmer le mot de passe"
              value={confirmation}
              set={setConfirmation}
              placeholder="Répétez le mot de passe"
              type="password"
              autoComplete="new-password"
            />
          )}
          {error && <div className="form-error">{error}</div>}
          <button className="button primary wide" disabled={busy}>
            {busy
              ? "Vérification…"
              : mode === "setup"
                ? "Créer mon espace"
                : "Continuer"}
            <Icon name="arrow" />
          </button>
          <div className="secure-note">
            <Icon name="lock" size={15} />
            <span>
              <strong>Protection locale</strong>PaieMaroc ne transmet jamais
              votre mot de passe.
            </span>
          </div>
        </form>
      </main>
    </div>
  );
}

function Workspace({ user, onLogout }: { user: User; onLogout: () => void }) {
  const [route, setRoute] = useState<Route>("overview"),
    [periods, setPeriods] = useState<Period[]>([]),
    [activePeriod, setActivePeriod] = useState(""),
    [employees, setEmployees] = useState<Employee[]>([]),
    [batch, setBatch] = useState<Batch>(blankBatch),
    [loading, setLoading] = useState(true),
    [message, setMessage] = useState("Données privées · stockage local"),
    [editing, setEditing] = useState<Employee | null>(null),
    [periodDialog, setPeriodDialog] = useState(false),
    [refresh, setRefresh] = useState(0);
  const period = periods.find((item) => item.period === activePeriod),
    canEdit = period?.status === "draft";
  async function loadPeriods(select?: string) {
    if (!isNative) {
      const demo = [
        {
          period: "2026-10",
          status: "draft",
          copiedFrom: "2026-09",
          employeeCount: 3,
          grossCentimes: 2980000,
          createdAt: "",
          closedAt: "",
        },
      ];
      setPeriods(demo);
      setActivePeriod("2026-10");
      return;
    }
    const result = (await ListPeriods()) as Period[];
    setPeriods(result);
    setActivePeriod(select || activePeriod || result[0]?.period || "");
  }
  useEffect(() => {
    loadPeriods()
      .catch((error) => setMessage(friendlyError(error)))
      .finally(() => setLoading(false));
  }, []);
  useEffect(() => {
    if (!activePeriod) {
      setEmployees([]);
      setBatch(blankBatch);
      return;
    }
    const load = async () => {
      const list = !isNative
        ? demoEmployees
        : ((await ListPeriodEmployees(activePeriod, "", "", 200))
            .items as Employee[]);
      setEmployees(list);
      if (!list.length) {
        setBatch(blankBatch);
        return;
      }
      try {
        setBatch(
          (isNative
            ? await CalculatePayroll(list as never)
            : localCalc(list)) as Batch,
        );
      } catch (error) {
        setMessage(friendlyError(error));
        setBatch(localCalc(list));
      }
    };
    load().catch((error) => setMessage(friendlyError(error)));
  }, [activePeriod, refresh]);
  async function saveEmployee(employee: Employee) {
    if (!activePeriod || !canEdit) {
      const reason =
        "Sélectionnez une période brouillon avant d’ajouter un salarié.";
      setMessage(reason);
      throw new Error(reason);
    }
    try {
      const alreadyExists = employees.some((item) => item.id === employee.id);
      const updatedEmployees = alreadyExists
        ? employees.map((item) => (item.id === employee.id ? employee : item))
        : [...employees, employee];
      if (isNative) await SavePeriodEmployee(activePeriod, employee as never);
      setEmployees(updatedEmployees);
      if (isNative) {
        await loadPeriods(activePeriod);
        setRefresh((value) => value + 1);
      } else {
        setBatch(localCalc(updatedEmployees));
        if (!alreadyExists) {
          setPeriods((current) =>
            current.map((item) =>
              item.period === activePeriod
                ? { ...item, employeeCount: item.employeeCount + 1 }
                : item,
            ),
          );
        }
      }
      setMessage(
        `${employee.firstName} ${employee.lastName} enregistré·e dans ${activePeriod}`,
      );
      setEditing(null);
    } catch (error) {
      const reason = friendlyError(error);
      setMessage(reason);
      throw new Error(reason);
    }
  }
  async function createPeriod(value: string, copyFrom: string) {
    try {
      if (isNative) await CreatePeriod({ period: value, copyFrom } as never);
      await loadPeriods(value);
      setPeriodDialog(false);
      setMessage(
        copyFrom
          ? `Paie ${value} créée à partir de ${copyFrom}`
          : `Paie ${value} créée`,
      );
    } catch (error) {
      setMessage(friendlyError(error));
    }
  }
  async function close() {
    if (!period) return;
    try {
      if (isNative) await ClosePeriod(period.period);
      await loadPeriods(period.period);
      setMessage(`Paie ${period.period} clôturée`);
    } catch (error) {
      setMessage(friendlyError(error));
    }
  }
  async function reopen() {
    if (!period) return;
    try {
      if (isNative) await ReopenPeriod(period.period);
      await loadPeriods(period.period);
      setMessage(`Paie ${period.period} rouverte`);
    } catch (error) {
      setMessage(friendlyError(error));
    }
  }
  async function removePeriod() {
    if (
      !period ||
      !confirm(`Supprimer définitivement la paie ${period.period} ?`)
    )
      return;
    try {
      if (isNative) await DeletePeriod(period.period);
      setActivePeriod("");
      await loadPeriods();
      setMessage(`Paie ${period.period} supprimée`);
    } catch (error) {
      setMessage(friendlyError(error));
    }
  }
  const navSections: {
    label: string;
    items: [Route, string, string][];
  }[] = [
    {
      label: "PILOTAGE",
      items: [
        ["overview", "Vue d’ensemble", "grid"],
        ["employees", "Collaborateurs", "users"],
        ["payroll", "Préparer la paie", "calc"],
        ["periods", "Mois & clôtures", "calendar"],
      ],
    },
    {
      label: "CONFORMITÉ & SORTIES",
      items: [
        ["cnss", "Déclaration CNSS", "shield"],
        ["exports", "Exports bancaires", "send"],
      ],
    },
    {
      label: "ATELIER",
      items: [["devices", "Pointeuses ZKTeco", "device"]],
    },
  ];
  if (user.role === "admin")
    navSections[2].items.push(["access", "Accès & utilisateurs", "key"]);
  const nav = navSections.flatMap((section) => section.items);
  const activeLabel = nav.find((item) => item[0] === route)?.[1] || "";
  const activeIndex = nav.findIndex((item) => item[0] === route) + 1;
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Brand light />
        <nav aria-label="Navigation principale">
          {navSections.map((section) => (
            <section key={section.label}>
              <small>{section.label}</small>
              {section.items.map(([value, label, icon]) => (
                <button
                  key={value}
                  className={route === value ? "active" : ""}
                  onClick={() => setRoute(value)}
                >
                  <span className="nav-code">
                    {String(
                      nav.findIndex((item) => item[0] === value) + 1,
                    ).padStart(2, "0")}
                  </span>
                  <Icon name={icon} />
                  <span>{label}</span>
                </button>
              ))}
            </section>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="local-pill">
            <Icon name="lock" />
            <span>
              <strong>Coffre de paie local</strong>
              <small>SQLite · cet appareil</small>
            </span>
            <em>LOCAL</em>
          </div>
          <button className="account" onClick={onLogout}>
            <b>{initials(user.displayName)}</b>
            <span>
              <strong>{user.displayName}</strong>
              <small>{roleName(user.role)}</small>
            </span>
            <Icon name="logout" />
          </button>
        </div>
      </aside>
      <main className="app-main">
        <header className="topbar">
          <div className="workspace-title">
            <span>PM/{String(activeIndex).padStart(2, "0")}</span>
            <strong>{activeLabel}</strong>
            <small>Gestion de paie marocaine</small>
          </div>
          <div className="top-actions">
            <span className="save-status">
              <Icon name="check" size={12} />
              Enregistré
            </span>
            <SelectField
              label="Période active"
              value={activePeriod}
              set={setActivePeriod}
              compact
              required={false}
            >
              <option value="">Choisir une période</option>
              {periods.map((item) => (
                <option key={item.period} value={item.period}>
                  {item.period} ·{" "}
                  {item.status === "closed" ? "Clôturée" : "Brouillon"}
                </option>
              ))}
            </SelectField>
            <button
              className="button primary"
              onClick={() => setPeriodDialog(true)}
            >
              <Icon name="plus" />
              Nouveau mois
            </button>
          </div>
        </header>
        <div className="workspace">
          {loading ? (
            <div className="panel loading">
              <span className="spinner" />
              Chargement…
            </div>
          ) : route === "access" ? (
            <AccessPage current={user} notify={setMessage} />
          ) : route === "devices" ? (
            <DevicesPage employees={employees} notify={setMessage} />
          ) : route === "periods" ? (
            <PeriodsPage
              periods={periods}
              active={activePeriod}
              select={setActivePeriod}
              create={() => setPeriodDialog(true)}
              reopen={reopen}
              remove={removePeriod}
            />
          ) : (
            <PayrollPage
              route={route}
              period={period}
              employees={employees}
              batch={batch}
              canEdit={Boolean(canEdit)}
              edit={setEditing}
              close={close}
              reopen={reopen}
              notify={setMessage}
              imported={() => {
                setRefresh((value) => value + 1);
                loadPeriods(activePeriod).catch((error) =>
                  setMessage(friendlyError(error)),
                );
              }}
            />
          )}
          <footer className="statusbar">
            <span>
              <Icon name="database" size={14} />
              SQLite WAL · pagination bornée · calcul Go
            </span>
            <strong>{message}</strong>
          </footer>
        </div>
      </main>
      {editing && (
        <EmployeeDialog
          value={editing}
          close={() => setEditing(null)}
          save={saveEmployee}
        />
      )}{" "}
      {periodDialog && (
        <PeriodDialog
          periods={periods}
          close={() => setPeriodDialog(false)}
          create={createPeriod}
        />
      )}
    </div>
  );
}

function PayrollPage({
  route,
  period,
  employees,
  batch,
  canEdit,
  edit,
  close,
  reopen,
  notify,
  imported,
}: {
  route: Route;
  period?: Period;
  employees: Employee[];
  batch: Batch;
  canEdit: boolean;
  edit: (value: Employee) => void;
  close: () => void;
  reopen: () => void;
  notify: (value: string) => void;
  imported: () => void;
}) {
  const [search, setSearch] = useState(""),
    [busy, setBusy] = useState(false),
    [format, setFormat] = useState("simt500");
  const rows = useMemo(() => {
    const query = search.toLowerCase().trim();
    return query
      ? batch.rows.filter((row) =>
          `${row.employee.firstName} ${row.employee.lastName} ${row.employee.id}`
            .toLowerCase()
            .includes(query),
        )
      : batch.rows;
  }, [batch, search]);
  function add() {
    if (canEdit)
      edit({
        id: nextEmployeeID(employees),
        firstName: "",
        lastName: "",
        cin: "",
        cnss: "",
        rib: "",
        daysWorked: 26,
        dependants: 0,
        grossCentimes: 600000,
        exemptCentimes: 0,
        advanceCentimes: 0,
      });
  }
  async function validate() {
    setBusy(true);
    try {
      const issues = isNative ? await ValidatePayroll(employees as never) : [];
      notify(
        issues.length
          ? `${issues.length} anomalie(s) à corriger`
          : `${employees.length} salariés contrôlés · aucune anomalie`,
      );
    } catch (error) {
      notify(friendlyError(error));
    } finally {
      setBusy(false);
    }
  }
  async function importCSV() {
    if (!period || !canEdit) return;
    setBusy(true);
    try {
      if (!isNative) {
        notify("Import CSV mensuel disponible dans l’application bureau");
        return;
      }
      const result = await ImportEmployeesCSVToPeriod(period.period);
      notify(
        `${result.imported.toLocaleString("fr")} salarié(s) importé(s) · ${result.rejected} rejet(s)`,
      );
      imported();
    } catch (error) {
      notify(friendlyError(error));
    } finally {
      setBusy(false);
    }
  }
  async function cnss() {
    setBusy(true);
    try {
      if (!isNative) {
        notify("Assistant e-BDS disponible dans l’application bureau");
        return;
      }
      const result = await GenerateCNSSBDS();
      notify(
        `BDS généré · ${result.employeeCount} salariés · ${result.entrantCount} entrant(s)`,
      );
    } catch (error) {
      notify(friendlyError(error));
    } finally {
      setBusy(false);
    }
  }
  async function exportFile() {
    if (!period) return;
    setBusy(true);
    const input = {
      format,
      company: {
        name: "Mon entreprise",
        affiliateCnss: "1773341",
        rib: "011780000000000000000123",
        participant: "011",
      },
      employees,
      period: period.period,
      executionDate: `${period.period}-28`,
      batchNumber: 1,
    };
    try {
      if (!isNative) {
        notify(`Aperçu ${format.toUpperCase()} prêt`);
        return;
      }
      const preview = await PreviewExport(input as never);
      notify(`${preview.name} contrôlé · ${money(preview.totalCentimes)}`);
      await SaveExport(input as never);
    } catch (error) {
      notify(friendlyError(error));
    } finally {
      setBusy(false);
    }
  }
  if (!period) return <EmptyState />;
  const active = period.period;
  return (
    <>
      <div className="page-title">
        <div>
          <span className="eyebrow">PAIE MENSUELLE · {active}</span>
          <h1>
            {route === "overview"
              ? `Votre paie ${active} est prête à avancer.`
              : route === "employees"
                ? "Collaborateurs"
                : route === "cnss"
                  ? "Déclaration e-BDS"
                  : route === "exports"
                    ? "Exports bancaires"
                    : "Préparer la paie"}
          </h1>
          <p>
            {period.status === "closed"
              ? "Cette période est clôturée. Rouvrez-la pour apporter une correction."
              : "Les modifications sont enregistrées uniquement dans ce mois."}
          </p>
        </div>
        <div>
          {period.status === "closed" ? (
            <button className="button secondary" onClick={reopen}>
              Rouvrir la période
            </button>
          ) : (
            <>
              <button
                className="button secondary"
                onClick={validate}
                disabled={busy}
              >
                Contrôler
              </button>
              <button className="button primary" onClick={close}>
                Clôturer le mois
              </button>
            </>
          )}
        </div>
      </div>
      {period.status === "closed" && (
        <div className="banner brand">
          <Icon name="lock" />
          <div>
            <strong>Période clôturée</strong>
            <small>
              Les exports restent accessibles; toute modification exige une
              réouverture.
            </small>
          </div>
        </div>
      )}
      <div className="metrics">
        <Metric
          label="NET À PAYER"
          value={money(batch.netTotalCentimes)}
          detail="Virements du mois"
          tone="brand"
          icon="send"
        />
        <Metric
          label="MASSE SALARIALE"
          value={money(batch.grossTotalCentimes)}
          detail="Brut mensuel"
          tone="blue"
          icon="calc"
        />
        <Metric
          label="CHARGES EMPLOYEUR"
          value={money(batch.employerChargesTotal)}
          detail="CNSS · AMO · AF · TFP"
          tone="amber"
          icon="shield"
        />
        <Metric
          label="COLLABORATEURS"
          value={String(period.employeeCount || employees.length)}
          detail={`${employees.length} chargé(s) à l’écran`}
          tone="green"
          icon="users"
        />
      </div>
      {route === "cnss" ? (
        <ActionPanel
          icon="shield"
          title="Générer la déclaration e-BDS"
          text="Chargez le préétabli CNSS et le CSV mensuel. PaieMaroc contrôle les entrants B04 et les totaux B00–B06."
          button="Ouvrir l’assistant CNSS"
          action={cnss}
          busy={busy}
        />
      ) : route === "exports" ? (
        <section className="panel export-panel">
          <div>
            <span className="action-icon">
              <Icon name="send" />
            </span>
            <div>
              <h2>Remise bancaire</h2>
              <p>Choisissez uniquement le format confirmé par votre banque.</p>
            </div>
          </div>
          <div className="format-picker">
            {[
              ["simt500", "SIMT 500", "TXT"],
              ["csv", "CSV universel", "CSV"],
              ["iso20022", "ISO 20022", "XML"],
            ].map((item) => (
              <button
                key={item[0]}
                className={format === item[0] ? "selected" : ""}
                onClick={() => setFormat(item[0])}
              >
                <small>{item[2]}</small>
                {item[1]}
              </button>
            ))}
          </div>
          <button
            className="button primary"
            onClick={exportFile}
            disabled={busy}
          >
            Contrôler et exporter
            <Icon name="arrow" />
          </button>
        </section>
      ) : (
        <>
          <section className="panel table-panel">
            <div className="panel-head">
              <div>
                <h2>Paie de {active}</h2>
                <p>Une fiche salarié, des valeurs mensuelles indépendantes.</p>
              </div>
              <div className="table-actions">
                <SearchField value={search} set={setSearch} />
                <button
                  className="button secondary"
                  onClick={importCSV}
                  disabled={!canEdit || busy}
                >
                  <Icon name="database" />
                  Importer CSV
                </button>
                <button
                  className="button primary"
                  onClick={add}
                  disabled={!canEdit}
                >
                  <Icon name="plus" />
                  Ajouter
                </button>
              </div>
            </div>
            <div className="table-scroll">
              <table>
                <thead>
                  <tr>
                    <th>COLLABORATEUR</th>
                    <th>JOURS</th>
                    <th>BRUT</th>
                    <th>RETENUES</th>
                    <th>NET</th>
                    <th>ÉTAT</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {rows.map((row) => (
                    <tr key={row.employee.id}>
                      <td>
                        <b className="avatar">
                          {initials(
                            `${row.employee.firstName} ${row.employee.lastName}`,
                          )}
                        </b>
                        <span>
                          <strong>
                            {row.employee.firstName} {row.employee.lastName}
                          </strong>
                          <small>
                            {row.employee.id} · CNSS {row.employee.cnss}
                          </small>
                        </span>
                      </td>
                      <td>{row.employee.daysWorked}</td>
                      <td>{money(row.employee.grossCentimes)}</td>
                      <td className="negative">
                        −{" "}
                        {money(
                          row.employeeCnss +
                            row.employeeAmo +
                            row.incomeTax +
                            row.employee.advanceCentimes,
                        )}
                      </td>
                      <td>
                        <strong>{money(row.netPay)}</strong>
                      </td>
                      <td>
                        <em className="badge success">Prêt</em>
                      </td>
                      <td>
                        <button
                          className="icon-button"
                          disabled={!canEdit}
                          onClick={() => edit(row.employee)}
                        >
                          <Icon name="more" />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {rows.length === 0 && (
                <div className="empty-table">
                  Aucun salarié dans cette période.
                </div>
              )}
            </div>
            <div className="panel-foot">
              <span>
                {rows.length} affiché(s) sur {period.employeeCount}
              </span>
              <span>Double contrôle avant clôture</span>
            </div>
          </section>
          <section className="flow-grid">
            <Flow
              number="01"
              icon="users"
              title="Fiches uniques"
              text="Ajoutez chaque salarié une seule fois, puis modifiez sa fiche."
            />
            <Flow
              number="02"
              icon="calendar"
              title="Valeurs par mois"
              text="Salaires, jours et avances restent attachés à leur période."
            />
            <Flow
              number="03"
              icon="check"
              title="Clôture réversible"
              text="Contrôlez, clôturez, puis rouvrez avec un droit administrateur."
            />
          </section>
        </>
      )}
    </>
  );
}

function PeriodsPage({
  periods,
  active,
  select,
  create,
  reopen,
  remove,
}: {
  periods: Period[];
  active: string;
  select: (value: string) => void;
  create: () => void;
  reopen: () => void;
  remove: () => void;
}) {
  const selected = periods.find((item) => item.period === active);
  return (
    <>
      <div className="page-title">
        <div>
          <span className="eyebrow">HISTORIQUE DE PAIE</span>
          <h1>Mois & clôtures</h1>
          <p>
            Revenez sur n’importe quel mois ou repartez de la paie précédente.
          </p>
        </div>
        <button className="button primary" onClick={create}>
          <Icon name="plus" />
          Nouveau mois
        </button>
      </div>
      <section className="period-layout">
        <div className="panel period-list">
          {periods.map((item) => (
            <button
              key={item.period}
              className={item.period === active ? "selected" : ""}
              onClick={() => select(item.period)}
            >
              <span className="period-icon">
                <Icon name="calendar" />
              </span>
              <span>
                <strong>{item.period}</strong>
                <small>
                  {item.copiedFrom
                    ? `Repris depuis ${item.copiedFrom}`
                    : "Créé depuis le référentiel"}
                </small>
              </span>
              <em
                className={`badge ${item.status === "closed" ? "neutral" : "success"}`}
              >
                {item.status === "closed" ? "Clôturée" : "Brouillon"}
              </em>
              <b>{money(item.grossCentimes)}</b>
            </button>
          ))}
        </div>
        {selected && (
          <aside className="panel period-detail">
            <span className="eyebrow">PÉRIODE SÉLECTIONNÉE</span>
            <h2>{selected.period}</h2>
            <dl>
              <div>
                <dt>État</dt>
                <dd>
                  {selected.status === "closed" ? "Clôturée" : "Brouillon"}
                </dd>
              </div>
              <div>
                <dt>Salariés</dt>
                <dd>{selected.employeeCount}</dd>
              </div>
              <div>
                <dt>Masse brute</dt>
                <dd>{money(selected.grossCentimes)}</dd>
              </div>
              <div>
                <dt>Source</dt>
                <dd>{selected.copiedFrom || "Référentiel"}</dd>
              </div>
            </dl>
            {selected.status === "closed" && (
              <button className="button secondary wide" onClick={reopen}>
                Rouvrir pour modifier
              </button>
            )}
            <button className="button danger wide" onClick={remove}>
              <Icon name="trash" />
              Supprimer ce mois
            </button>
            <p className="danger-note">
              La suppression efface uniquement cette paie mensuelle, jamais les
              fiches salariés.
            </p>
          </aside>
        )}
      </section>
    </>
  );
}

function AccessPage({
  current,
  notify,
}: {
  current: User;
  notify: (value: string) => void;
}) {
  const [users, setUsers] = useState<User[]>([]),
    [displayName, setDisplayName] = useState(""),
    [username, setUsername] = useState(""),
    [password, setPassword] = useState(""),
    [role, setRole] = useState("manager"),
    [reset, setReset] = useState<User | null>(null),
    [resetPassword, setResetPassword] = useState("");
  const load = () =>
    isNative
      ? ListUsers().then((value) => setUsers(value as User[]))
      : Promise.resolve(
          setUsers([
            demoUser,
            {
              ...demoUser,
              id: 2,
              username: "salma.rh",
              displayName: "Salma RH",
              role: "manager",
            },
          ]),
        );
  useEffect(() => {
    load().catch((error) => notify(friendlyError(error)));
  }, []);
  async function add(event: FormEvent) {
    event.preventDefault();
    try {
      if (isNative)
        await CreateUser({ displayName, username, password, role } as never);
      const created = username;
      setDisplayName("");
      setUsername("");
      setPassword("");
      await load();
      notify(`Compte ${created} créé`);
    } catch (error) {
      notify(friendlyError(error));
    }
  }
  async function toggle(user: User) {
    try {
      if (isNative) await SetUserEnabled(user.id, !user.enabled);
      await load();
      notify(`${user.displayName} ${user.enabled ? "désactivé" : "réactivé"}`);
    } catch (error) {
      notify(friendlyError(error));
    }
  }
  async function saveReset(event: FormEvent) {
    event.preventDefault();
    if (!reset) return;
    try {
      if (isNative) await ResetUserPassword(reset.id, resetPassword);
      notify(`Mot de passe de ${reset.displayName} réinitialisé`);
      setReset(null);
      setResetPassword("");
    } catch (error) {
      notify(friendlyError(error));
    }
  }
  return (
    <>
      <div className="page-title">
        <div>
          <span className="eyebrow">ADMINISTRATION</span>
          <h1>Accès & utilisateurs</h1>
          <p>
            Donnez uniquement les droits nécessaires à chaque membre de
            l’équipe.
          </p>
        </div>
      </div>
      <div className="access-layout">
        <section className="panel users-panel">
          <div className="panel-head">
            <div>
              <h2>Utilisateurs</h2>
              <p>
                {users.filter((user) => user.enabled).length} compte(s) actif(s)
              </p>
            </div>
          </div>
          {users.map((user) => (
            <div className="user-row" key={user.id}>
              <b className="avatar large">{initials(user.displayName)}</b>
              <span>
                <strong>
                  {user.displayName}
                  {user.id === current.id && <em>Vous</em>}
                </strong>
                <small>
                  @{user.username} · {roleName(user.role)}
                </small>
              </span>
              <em className={`badge ${user.enabled ? "success" : "neutral"}`}>
                {user.enabled ? "Actif" : "Suspendu"}
              </em>
              <button
                className="icon-button labeled"
                onClick={() => setReset(user)}
              >
                <Icon name="key" />
                Mot de passe
              </button>
              <button
                className="button secondary small"
                disabled={user.id === current.id}
                onClick={() => toggle(user)}
              >
                {user.enabled ? "Désactiver" : "Réactiver"}
              </button>
            </div>
          ))}
        </section>
        <form className="panel invite-card" onSubmit={add}>
          <span className="eyebrow">NOUVEL ACCÈS</span>
          <h2>Ajouter un utilisateur</h2>
          <p>
            Le mot de passe temporaire est communiqué hors de l’application.
          </p>
          <TextField
            label="Nom complet"
            value={displayName}
            set={setDisplayName}
            placeholder="Salma Alaoui"
          />
          <TextField
            label="Identifiant"
            value={username}
            set={setUsername}
            placeholder="salma.rh"
          />
          <TextField
            label="Mot de passe temporaire"
            value={password}
            set={setPassword}
            type="password"
            placeholder="12 caractères minimum"
          />
          <SelectField label="Rôle" value={role} set={setRole}>
            <option value="manager">Gestionnaire paie</option>
            <option value="admin">Administrateur</option>
          </SelectField>
          <button className="button primary wide">
            Créer le compte
            <Icon name="arrow" />
          </button>
        </form>
      </div>
      {reset && (
        <div className="backdrop">
          <form className="dialog compact" onSubmit={saveReset}>
            <DialogHead
              eyebrow="SÉCURITÉ"
              title={`Réinitialiser ${reset.displayName}`}
              close={() => setReset(null)}
            />
            <TextField
              label="Nouveau mot de passe"
              value={resetPassword}
              set={setResetPassword}
              type="password"
              placeholder="12 caractères minimum"
            />
            <div className="dialog-actions">
              <button
                type="button"
                className="button secondary"
                onClick={() => setReset(null)}
              >
                Annuler
              </button>
              <button className="button primary">Mettre à jour</button>
            </div>
          </form>
        </div>
      )}
    </>
  );
}

function DevicesPage({
  employees,
  notify,
}: {
  employees: Employee[];
  notify: (value: string) => void;
}) {
  const [devices, setDevices] = useState<Device[]>([]),
    [editing, setEditing] = useState<Device>(blankDevice),
    [profile, setProfile] = useState<{
      authentication: string[];
      faceCapacity: number;
      fingerCapacity: number;
      maxEvents: number;
    } | null>(null),
    [testing, setTesting] = useState(false),
    [linkEmployee, setLinkEmployee] = useState(""),
    [deviceUserID, setDeviceUserID] = useState("");
  const load = () =>
    isNative
      ? ListAttendanceDevices().then((value) => setDevices(value as Device[]))
      : Promise.resolve(
          setDevices([
            { ...blankDevice, id: 1, lastStatus: "online", linkedUsers: 3 },
          ]),
        );
  useEffect(() => {
    load().catch((error) => notify(friendlyError(error)));
    if (isNative)
      AttendanceProfiles().then((value) => setProfile(value[0] || null));
    else
      setProfile({
        authentication: ["visage", "empreinte", "badge optionnel"],
        faceCapacity: 100,
        fingerCapacity: 500,
        maxEvents: 50000,
      });
  }, []);
  async function save(event: FormEvent) {
    event.preventDefault();
    try {
      const saved = isNative
        ? await SaveAttendanceDevice(editing as never)
        : { ...editing, id: editing.id || 2 };
      setEditing(saved as Device);
      await load();
      notify(`${saved.name} enregistrée`);
    } catch (error) {
      notify(friendlyError(error));
    }
  }
  async function test() {
    setTesting(true);
    try {
      if (!isNative) {
        notify("MB10-VL joignable · 4 ms");
        return;
      }
      const result = await TestAttendanceDevice(editing as never);
      notify(`${result.message} ${result.latencyMs} ms`);
      await load();
    } catch (error) {
      notify(friendlyError(error));
    } finally {
      setTesting(false);
    }
  }
  async function link(event: FormEvent) {
    event.preventDefault();
    try {
      if (isNative)
        await LinkAttendanceEmployee(editing.id, linkEmployee, deviceUserID);
      notify(
        `Identifiant ZKTeco ${deviceUserID} lié au salarié ${linkEmployee}`,
      );
      setLinkEmployee("");
      setDeviceUserID("");
      await load();
    } catch (error) {
      notify(friendlyError(error));
    }
  }
  return (
    <>
      <div className="page-title">
        <div>
          <span className="eyebrow">TEMPS & PRÉSENCE</span>
          <h1>Pointeuses ZKTeco</h1>
          <p>
            Connectez plusieurs terminaux sans copier les empreintes ni les
            visages dans PaieMaroc.
          </p>
        </div>
        <button
          className="button primary"
          onClick={() => setEditing({ ...blankDevice })}
        >
          <Icon name="plus" />
          Ajouter une pointeuse
        </button>
      </div>
      <div className="banner blue">
        <Icon name="shield" />
        <div>
          <strong>Profil prioritaire MB10-VL</strong>
          <small>
            TCP/IP · ADMS · visage · empreinte · badge optionnel. Détection
            réseau avant synchronisation.
          </small>
        </div>
      </div>
      <div className="device-layout">
        <section className="panel device-list">
          <div className="panel-head">
            <div>
              <h2>Terminaux</h2>
              <p>{devices.length} pointeuse(s) configurée(s)</p>
            </div>
          </div>
          {devices.map((device) => (
            <button
              key={device.id}
              className={editing.id === device.id ? "selected" : ""}
              onClick={() => setEditing(device)}
            >
              <span className="device-symbol">
                <Icon name="device" />
              </span>
              <span>
                <strong>{device.name}</strong>
                <small>
                  {device.model} · {device.host}:{device.port}
                </small>
              </span>
              <em
                className={`badge ${device.lastStatus === "online" ? "success" : "neutral"}`}
              >
                {device.lastStatus === "online"
                  ? "En ligne"
                  : device.lastStatus === "offline"
                    ? "Hors ligne"
                    : "À tester"}
              </em>
              <b>{device.linkedUsers} lié(s)</b>
            </button>
          ))}
        </section>
        <form className="panel device-form" onSubmit={save}>
          <span className="eyebrow">CONFIGURATION</span>
          <h2>{editing.id ? "Modifier la pointeuse" : "Nouvelle pointeuse"}</h2>
          <div className="form-grid">
            <TextField
              label="Nom"
              value={editing.name}
              set={(value) => setEditing({ ...editing, name: value })}
            />
            <TextField
              label="Modèle"
              value={editing.model}
              set={(value) => setEditing({ ...editing, model: value })}
            />
            <SelectField
              label="Mode de connexion"
              value={editing.protocol}
              set={(value) => setEditing({ ...editing, protocol: value })}
              hint="Adaptateur utilisé pour dialoguer avec la pointeuse"
            >
              <option value="standalone">Standalone TCP/IP</option>
              <option value="adms">PUSH / ADMS</option>
              <option value="biotime">API ZKBio Time</option>
            </SelectField>
            <TextField
              label="Adresse IP / hôte"
              value={editing.host}
              set={(value) => setEditing({ ...editing, host: value })}
            />
            <TextField
              label="Port"
              value={String(editing.port)}
              set={(value) => setEditing({ ...editing, port: Number(value) })}
            />
          </div>
          <div className="device-capabilities">
            <span>
              <Icon name="users" />
              <strong>{profile?.faceCapacity || 100}</strong> visages
            </span>
            <span>
              <Icon name="users" />
              <strong>{profile?.fingerCapacity || 500}</strong> empreintes
            </span>
            <span>
              <Icon name="clock" />
              <strong>
                {(profile?.maxEvents || 50000).toLocaleString("fr")}
              </strong>{" "}
              événements
            </span>
            <span>
              <Icon name="shield" />
              <strong>
                {profile?.authentication.join(" · ") || "Visage · empreinte"}
              </strong>
            </span>
          </div>
          <div className="dialog-actions split">
            <button
              type="button"
              className="button secondary"
              onClick={test}
              disabled={testing}
            >
              {testing ? "Test…" : "Tester la connexion"}
            </button>
            <button className="button primary">
              Enregistrer
              <Icon name="arrow" />
            </button>
          </div>
          {editing.id > 0 && (
            <div className="link-form">
              <div>
                <strong>Lier un salarié</strong>
                <small>ID numérique, sans zéro initial.</small>
              </div>
              <SelectField
                label="Salarié"
                value={linkEmployee}
                set={setLinkEmployee}
                compact
              >
                <option value="">Choisir…</option>
                {employees.map((employee) => (
                  <option key={employee.id} value={employee.id}>
                    {employee.firstName} {employee.lastName}
                  </option>
                ))}
              </SelectField>
              <TextField
                label="ID pointeuse"
                value={deviceUserID}
                set={setDeviceUserID}
                placeholder="ID ZKTeco"
                compact
              />
              <button
                type="button"
                className="button secondary"
                onClick={(event) => link(event as unknown as FormEvent)}
              >
                Lier
              </button>
            </div>
          )}
        </form>
      </div>
      <section className="panel protocol-note">
        <Icon name="database" />
        <div>
          <strong>Compatibilité par adaptateurs</strong>
          <p>
            MB10-VL : Standalone/ADMS. Autres familles : PUSH/ADMS ou ZKBio Time
            selon le firmware. Les gabarits biométriques ne quittent jamais la
            pointeuse.
          </p>
        </div>
      </section>
    </>
  );
}

function EmployeeDialog({
  value,
  close,
  save,
}: {
  value: Employee;
  close: () => void;
  save: (value: Employee) => Promise<void>;
}) {
  const [employee, setEmployee] = useState(value),
    [error, setError] = useState(""),
    [errors, setErrors] = useState<EmployeeErrors>({}),
    [busy, setBusy] = useState(false);
  const update = (next: Employee, field: keyof Employee) => {
    setEmployee(next);
    setErrors((current) => ({ ...current, [field]: undefined }));
    if (error) setError("");
  };
  const focusField = (field: keyof Employee) =>
    window.requestAnimationFrame(() =>
      document.querySelector<HTMLInputElement>(`[name="${field}"]`)?.focus(),
    );
  return (
    <div
      className="backdrop"
      role="dialog"
      aria-modal="true"
      aria-label={
        value.firstName ? "Modifier le salarié" : "Ajouter un salarié"
      }
      onMouseDown={close}
    >
      <form
        className="dialog"
        noValidate
        onMouseDown={(event) => event.stopPropagation()}
        onSubmit={async (event) => {
          event.preventDefault();
          const issues = validateEmployee(employee);
          if (issues.length) {
            setErrors(
              Object.fromEntries(
                issues.map((issue) => [issue.field, issue.message]),
              ) as EmployeeErrors,
            );
            setError(
              issues.length === 1
                ? issues[0].message
                : `Corrigez les ${issues.length} champs signalés.`,
            );
            focusField(issues[0].field);
            return;
          }
          setBusy(true);
          try {
            await save(employee);
          } catch (reason) {
            const message = friendlyError(reason);
            const field = employeeFieldFromError(message);
            setError(message.replace(/^[^:]+:\s*/, ""));
            if (field) {
              setErrors((current) => ({
                ...current,
                [field]: message.replace(/^[^:]+:\s*/, ""),
              }));
              focusField(field);
            }
          } finally {
            setBusy(false);
          }
        }}
      >
        <DialogHead
          eyebrow="FICHE UNIQUE"
          title={
            employee.firstName
              ? `${employee.firstName} ${employee.lastName}`
              : "Nouveau collaborateur"
          }
          close={close}
        />
        <div className="form-grid two">
          <TextField
            name="firstName"
            label="Prénom"
            value={employee.firstName}
            set={(firstName) => update({ ...employee, firstName }, "firstName")}
            error={errors.firstName}
          />
          <TextField
            name="lastName"
            label="Nom"
            value={employee.lastName}
            set={(lastName) => update({ ...employee, lastName }, "lastName")}
            error={errors.lastName}
          />
          <TextField
            name="id"
            label="Matricule unique"
            hint="A–Z · 0–9 · . _ / -"
            value={employee.id}
            set={(id) => update({ ...employee, id: id.toUpperCase() }, "id")}
            readOnly={Boolean(value.firstName)}
            error={errors.id}
            pattern={employeePatterns.id}
            maxLength={32}
          />
          <TextField
            name="cin"
            label="CIN"
            hint="Ex. AB123456"
            value={employee.cin}
            set={(cin) =>
              update({ ...employee, cin: cin.toUpperCase() }, "cin")
            }
            error={errors.cin}
            pattern={employeePatterns.cin}
            maxLength={10}
          />
          <TextField
            name="cnss"
            label="N° CNSS"
            hint="6–12 chiffres"
            value={employee.cnss}
            set={(cnss) =>
              update({ ...employee, cnss: cnss.replace(/[^0-9]/g, "") }, "cnss")
            }
            error={errors.cnss}
            pattern={employeePatterns.cnss}
            maxLength={12}
            inputMode="numeric"
          />
          <TextField
            name="daysWorked"
            label="Jours ce mois"
            hint="0–30"
            type="number"
            value={String(employee.daysWorked)}
            set={(daysWorked) =>
              update(
                { ...employee, daysWorked: Number(daysWorked) },
                "daysWorked",
              )
            }
            error={errors.daysWorked}
            min={0}
            max={30}
          />
          <TextField
            name="dependants"
            label="Personnes à charge"
            hint="0–6"
            type="number"
            value={String(employee.dependants)}
            set={(dependants) =>
              update(
                { ...employee, dependants: Number(dependants) },
                "dependants",
              )
            }
            error={errors.dependants}
            min={0}
            max={6}
          />
          <div className="span-two">
            <TextField
              name="rib"
              label="RIB marocain · 24 chiffres"
              hint="Chiffres uniquement"
              value={employee.rib}
              set={(rib) =>
                update({ ...employee, rib: rib.replace(/[^0-9]/g, "") }, "rib")
              }
              error={errors.rib}
              pattern={employeePatterns.rib}
              maxLength={24}
              inputMode="numeric"
            />
          </div>
          <TextField
            name="grossCentimes"
            label="Salaire brut ce mois (MAD)"
            type="number"
            value={String(employee.grossCentimes / 100)}
            set={(value) =>
              update(
                {
                  ...employee,
                  grossCentimes: Math.round(Number(value) * 100),
                },
                "grossCentimes",
              )
            }
            error={errors.grossCentimes}
            min={0.01}
            step={0.01}
          />
          <TextField
            name="exemptCentimes"
            label="Montant exonéré (MAD)"
            type="number"
            value={String(employee.exemptCentimes / 100)}
            set={(value) =>
              update(
                {
                  ...employee,
                  exemptCentimes: Math.round(Number(value) * 100),
                },
                "exemptCentimes",
              )
            }
            error={errors.exemptCentimes}
            min={0}
            step={0.01}
          />
          <TextField
            name="advanceCentimes"
            label="Avance ce mois (MAD)"
            type="number"
            value={String(employee.advanceCentimes / 100)}
            set={(value) =>
              update(
                {
                  ...employee,
                  advanceCentimes: Math.round(Number(value) * 100),
                },
                "advanceCentimes",
              )
            }
            error={errors.advanceCentimes}
            min={0}
            step={0.01}
          />
        </div>
        {error && (
          <div className="form-error dialog-error" role="alert">
            <Icon name="warning" size={14} />
            {error}
          </div>
        )}
        <div className="secure-note">
          <Icon name="calendar" />
          <span>
            <strong>Flux mensuel</strong>L’identité est mise à jour une seule
            fois; les montants restent liés au mois actif.
          </span>
        </div>
        <div className="dialog-actions">
          <button type="button" className="button secondary" onClick={close}>
            Annuler
          </button>
          <button className="button primary" disabled={busy}>
            {busy ? "Enregistrement…" : "Enregistrer"}
          </button>
        </div>
      </form>
    </div>
  );
}
function PeriodDialog({
  periods,
  close,
  create,
}: {
  periods: Period[];
  close: () => void;
  create: (period: string, copy: string) => void;
}) {
  const now = new Date(),
    suggested = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`,
    [period, setPeriod] = useState(suggested),
    [source, setSource] = useState(periods[0]?.period || "");
  return (
    <div className="backdrop">
      <form
        className="dialog compact"
        onSubmit={(event) => {
          event.preventDefault();
          create(period, source);
        }}
      >
        <DialogHead
          eyebrow="NOUVELLE PAIE"
          title="Créer un mois"
          close={close}
        />
        <TextField
          label="Période"
          value={period}
          set={setPeriod}
          type="month"
        />
        <SelectField
          label="Reprendre les salaires de"
          value={source}
          set={setSource}
          required={false}
        >
          <option value="">Aucun mois · valeurs du référentiel</option>
          {periods.map((item) => (
            <option key={item.period} value={item.period}>
              {item.period}
            </option>
          ))}
        </SelectField>
        <div className="banner mini">
          <Icon name="check" />
          <div>
            <strong>Reprise intelligente</strong>
            <small>
              Salaires et jours sont copiés; les avances repartent à zéro.
            </small>
          </div>
        </div>
        <div className="dialog-actions">
          <button type="button" className="button secondary" onClick={close}>
            Annuler
          </button>
          <button className="button primary">Créer la période</button>
        </div>
      </form>
    </div>
  );
}
function Metric({
  label,
  value,
  detail,
  tone,
  icon,
}: {
  label: string;
  value: string;
  detail: string;
  tone: string;
  icon: string;
}) {
  return (
    <article className="panel metric">
      <div>
        <span>{label}</span>
        <i className={tone}>
          <Icon name={icon} />
        </i>
      </div>
      <strong>{value}</strong>
      <small>{detail}</small>
    </article>
  );
}
function Flow({
  number,
  icon,
  title,
  text,
}: {
  number: string;
  icon: string;
  title: string;
  text: string;
}) {
  return (
    <div className="panel flow-card">
      <span>{number}</span>
      <Icon name={icon} />
      <h3>{title}</h3>
      <p>{text}</p>
    </div>
  );
}
function TextField({
  name,
  label,
  value,
  set,
  placeholder = "",
  type = "text",
  autoComplete,
  readOnly = false,
  hint,
  compact = false,
  required = true,
  error,
  pattern,
  maxLength,
  inputMode,
  min,
  max,
  step,
}: {
  name?: string;
  label: string;
  value: string;
  set: (value: string) => void;
  placeholder?: string;
  type?: string;
  autoComplete?: string;
  readOnly?: boolean;
  hint?: string;
  compact?: boolean;
  required?: boolean;
  error?: string;
  pattern?: string;
  maxLength?: number;
  inputMode?:
    | "none"
    | "text"
    | "tel"
    | "url"
    | "email"
    | "numeric"
    | "decimal"
    | "search";
  min?: number;
  max?: number;
  step?: number;
}) {
  const id = useId(),
    errorID = `${id}-error`;
  return (
    <label
      className={`field pm-control ${compact ? "compact-control" : ""} ${error ? "invalid" : ""}`}
      htmlFor={id}
    >
      <span className="control-label">
        {label}
        {hint && <small>{hint}</small>}
      </span>
      <span className="control-shell">
        <input
          id={id}
          name={name}
          type={type}
          value={value}
          onChange={(event) => set(event.target.value)}
          placeholder={placeholder}
          autoComplete={autoComplete}
          readOnly={readOnly}
          required={required}
          aria-invalid={Boolean(error)}
          aria-describedby={error ? errorID : undefined}
          pattern={pattern}
          maxLength={maxLength}
          inputMode={inputMode}
          min={min}
          max={max}
          step={step}
        />
        <span className="control-stamp" aria-hidden="true">
          {readOnly ? "LIÉ" : type === "month" ? "MOIS" : "PM"}
        </span>
      </span>
      {error && (
        <small className="field-error" id={errorID}>
          {error}
        </small>
      )}
    </label>
  );
}

function SelectField({
  label,
  value,
  set,
  children,
  hint,
  compact = false,
  required = true,
}: {
  label: string;
  value: string;
  set: (value: string) => void;
  children: ReactNode;
  hint?: string;
  compact?: boolean;
  required?: boolean;
}) {
  const id = useId();
  return (
    <label
      className={`field pm-control select-control ${compact ? "compact-control" : ""}`}
      htmlFor={id}
    >
      <span className="control-label">
        {label}
        {hint && <small>{hint}</small>}
      </span>
      <span className="control-shell">
        <select
          id={id}
          value={value}
          onChange={(event) => set(event.target.value)}
          required={required}
        >
          {children}
        </select>
        <span className="select-glyph" aria-hidden="true">
          <i />
        </span>
      </span>
    </label>
  );
}

function SearchField({
  value,
  set,
}: {
  value: string;
  set: (value: string) => void;
}) {
  const id = useId();
  return (
    <div className="search-control" role="search">
      <label className="visually-hidden" htmlFor={id}>
        Rechercher un salarié
      </label>
      <Icon name="search" />
      <span>
        <small>RECHERCHE</small>
        <input
          id={id}
          value={value}
          onChange={(event) => set(event.target.value)}
          placeholder="Nom, CIN ou CNSS"
        />
      </span>
      {value && (
        <button
          type="button"
          onClick={() => set("")}
          aria-label="Effacer la recherche"
        >
          ×
        </button>
      )}
    </div>
  );
}
function DialogHead({
  eyebrow,
  title,
  close,
}: {
  eyebrow: string;
  title: string;
  close: () => void;
}) {
  return (
    <div className="dialog-head">
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h2>{title}</h2>
      </div>
      <button type="button" onClick={close}>
        ×
      </button>
    </div>
  );
}
function ActionPanel({
  icon,
  title,
  text,
  button,
  action,
  busy,
}: {
  icon: string;
  title: string;
  text: string;
  button: string;
  action: () => void;
  busy: boolean;
}) {
  return (
    <section className="panel action-panel">
      <span className="action-icon">
        <Icon name={icon} />
      </span>
      <div>
        <h2>{title}</h2>
        <p>{text}</p>
      </div>
      <button className="button primary" onClick={action} disabled={busy}>
        {busy ? "Traitement…" : button}
        <Icon name="arrow" />
      </button>
    </section>
  );
}
function EmptyState() {
  return (
    <section className="panel empty-state">
      <span>
        <Icon name="calendar" size={26} />
      </span>
      <h2>Commencez par créer un mois de paie</h2>
      <p>
        Chaque mois garde ses propres salaires, jours, avances et état de
        clôture.
      </p>
      <button
        className="button primary"
        onClick={() =>
          document
            .querySelector<HTMLButtonElement>(".top-actions .primary")
            ?.click()
        }
      >
        Créer un mois
        <Icon name="arrow" />
      </button>
    </section>
  );
}
function Brand({ light = false }: { light?: boolean }) {
  return (
    <div className={`brand ${light ? "light" : ""}`}>
      <img
        src={light ? whiteBrandLogo : brandLogo}
        alt="PaieMaroc — Gestion des Ressources Humaines"
      />
    </div>
  );
}
function CenteredError({ message }: { message: string }) {
  return (
    <div className="boot error">
      <Brand />
      <h1>Impossible d’ouvrir PaieMaroc</h1>
      <p>{message}</p>
    </div>
  );
}
