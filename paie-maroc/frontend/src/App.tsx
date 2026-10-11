import { useEffect, useId, useMemo, useState } from "react";
import type { FormEvent, ReactElement, ReactNode } from "react";
import {
  AttendanceProfiles,
  CalculatePayroll,
  ClosePeriod,
  CreateInitialAdmin,
  CreatePeriod,
  CreateUser,
  DeleteAttendanceHoliday,
  DeletePeriod,
  GenerateCNSSBDS,
  GeneratePointerWireGuardIdentity,
  GetAttendanceWorkPolicy,
  GetAuthState,
  GetPointerConnectivity,
  ImportEmployeesCSVToPeriod,
  LinkAttendanceEmployee,
  ListAttendanceDevices,
  ListAttendanceHolidays,
  ListAttendancePunchChanges,
  ListAttendancePunches,
  ListPeriodEmployees,
  ListPeriods,
  ListUsers,
  Login,
  Logout,
  PreviewExport,
  ReopenPeriod,
  ResetUserPassword,
  SaveAttendanceDevice,
  SaveAttendanceExport,
  SaveAttendanceHoliday,
  SaveAttendancePunch,
  SaveAttendanceWorkPolicy,
  SaveExport,
  SavePeriodEmployee,
  SavePointerConnectivity,
  SetUserEnabled,
  SyncAttendanceDevice,
  TestAttendanceDevice,
  TestPointerWireGuard,
  ValidatePayroll,
} from "../wailsjs/go/main/App";
import {
  Quit,
  WindowMinimise,
  WindowToggleMaximise,
} from "../wailsjs/runtime/runtime";
import brandLogo from "./assets/paie-maroc-hr-logo.png";
import whiteBrandLogo from "./assets/paie-maroc-hr-logo-white.png";
import {
  buildCNSSReadiness,
  describeCNSSError,
  type CNSSErrorNotice,
  type CNSSReadiness,
} from "./cnss-guidance";
import {
  employeePatterns,
  nextEmployeeID,
  validateEmployee,
} from "./employee-validation";
import type { Employee } from "./employee-validation";
import {DateField, MonthField} from "./date-picker";
import { parseWireGuardConfig } from "./wireguard-config";
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
type CNSSResult = {
  path: string;
  imported: number;
  existingCount: number;
  entrantCount: number;
  employeeCount: number;
  grossCentimes: number;
  cappedCentimes: number;
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
  transport: "lan" | "wireguard";
  serialNumber: string;
  host: string;
  port: number;
  enabled: boolean;
  lastStatus: string;
  lastCheckedAt: string;
  linkedUsers: number;
};
type PointerConnectivityConfig = {
  hook: {
    enabled: boolean;
    bindAddress: string;
    port: number;
    allowedNetworks: string[];
    firewallConsent: boolean;
  };
  wireGuard: {
    enabled: boolean;
    address: string;
    privateKey?: string;
    peerPublicKey: string;
    presharedKey?: string;
    endpoint: string;
    allowedIPs: string[];
    persistentKeepaliveSeconds: number;
    mtu: number;
  };
};
type PointerConnectivityStatus = {
  collectorActive: boolean;
  wireGuardActive: boolean;
  wireGuardHandshake: boolean;
  wireGuardResolvedEndpoint: string;
  wireGuardLastHandshakeAt: string;
  wireGuardReceivedBytes: number;
  wireGuardSentBytes: number;
  lanAddress: string;
  tunnelAddress: string;
  firewallGuidance: string;
  lastError: string;
  config: {
    hook: PointerConnectivityConfig["hook"];
    wireGuard: Omit<PointerConnectivityConfig["wireGuard"], "privateKey" | "presharedKey"> & {
      hasPrivateKey: boolean;
      publicKey: string;
      hasPresharedKey: boolean;
    };
  };
};
type PointerConnectivityTest = {
  target: string;
  reachable: boolean;
  latencyMilliseconds: number;
  handshakeEstablished: boolean;
  lastHandshakeAt: string;
  resolvedEndpoint: string;
  receivedBytes: number;
  sentBytes: number;
  message: string;
};
type Punch = {
  id: number;
  employeeId: string;
  employeeName: string;
  deviceId: number;
  deviceName: string;
  deviceUserId: string;
  occurredAt: string;
  direction: "entry" | "exit" | "unknown";
  verification: string;
  source: "device" | "manual";
  note: string;
  edited: boolean;
  updatedAt: string;
  unlinked: boolean;
};
type PunchFilter = {
  from: string;
  to: string;
  employeeId: string;
  deviceId: number;
  query: string;
  cursor: number;
  limit: number;
};
type PunchPage = {
  items: Punch[];
  nextCursor: number;
  total: number;
  unlinked: number;
};
type Holiday = {
  date: string;
  name: string;
  kind: "national" | "religious" | "company";
  paid: boolean;
  premiumBasisPoints: number;
  status: "confirmed" | "provisional";
  source: string;
};
type WorkPolicy = {
  sector: "non_agricultural" | "agricultural";
  weeklyMinutes: number;
  dayStart: string;
  dayEnd: string;
  breakMinutes: number;
  graceMinutes: number;
  dayPremiumBasisPoints: number;
  nightPremiumBasisPoints: number;
  restDayPremiumBasisPoints: number;
  restNightPremiumBasisPoints: number;
  rulesVersion: string;
};
type Route =
  | "overview"
  | "employees"
  | "payroll"
  | "cnss"
  | "exports"
  | "periods"
  | "attendance"
  | "devices"
  | "access";

const isNative = Boolean((window as unknown as { go?: unknown }).go);
const isMac = /Mac/i.test(navigator.platform);
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
  transport: "lan",
  serialNumber: "",
  host: "192.168.1.201",
  port: 4370,
  enabled: true,
  lastStatus: "never",
  lastCheckedAt: "",
  linkedUsers: 0,
};
const defaultWorkPolicy: WorkPolicy = {
  sector: "non_agricultural",
  weeklyMinutes: 2640,
  dayStart: "06:00",
  dayEnd: "21:00",
  breakMinutes: 60,
  graceMinutes: 5,
  dayPremiumBasisPoints: 2500,
  nightPremiumBasisPoints: 5000,
  restDayPremiumBasisPoints: 5000,
  restNightPremiumBasisPoints: 10000,
  rulesVersion: "MA-CODE-TRAVAIL-184-201",
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
      items: [
        ["attendance", "Journal de pointage", "clock"],
        ["devices", "Pointeuses ZKTeco", "device"],
      ],
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
            <DevicesPage
              employees={employees}
              isAdmin={user.role === "admin"}
              notify={setMessage}
            />
          ) : route === "attendance" ? (
            <AttendancePage
              employees={employees}
              activePeriod={activePeriod}
              isAdmin={user.role === "admin"}
              notify={setMessage}
            />
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
    [format, setFormat] = useState("simt500"),
    [cnssResult, setCNSSResult] = useState<CNSSResult | null>(null),
    [cnssIssue, setCNSSIssue] = useState<CNSSErrorNotice | null>(null);
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
  const cnssReadiness = useMemo(
    () =>
      buildCNSSReadiness(
        employees,
        period?.status ?? "open",
        period?.employeeCount ?? employees.length,
      ),
    [employees, period?.employeeCount, period?.status],
  );
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
    if (!period) return;
    if (!cnssReadiness.canGenerate) {
      setCNSSIssue({
        code: "BDS-010",
        title: "Précontrôle bloquant",
        detail: `Corrigez ${cnssReadiness.errors.length} anomalie(s) affichée(s) avant de choisir le préétabli.`,
        cancelled: false,
      });
      notify("BDS bloqué · corrigez les anomalies affichées");
      return;
    }
    setBusy(true);
    setCNSSIssue(null);
    setCNSSResult(null);
    try {
      if (!isNative) {
        setCNSSIssue({
          code: "BDS-DEMO",
          title: "Aperçu du parcours",
          detail:
            "La sélection du préétabli et l’écriture du fichier sont disponibles dans l’application bureau.",
          cancelled: true,
        });
        notify("Assistant e-BDS disponible dans l’application bureau");
        return;
      }
      const result = (await GenerateCNSSBDS(period.period)) as CNSSResult;
      setCNSSResult(result);
      notify(
        `BDS généré · ${result.employeeCount} salariés · ${result.entrantCount} entrant(s)`,
      );
    } catch (error) {
      const issue = describeCNSSError(error);
      setCNSSIssue(issue);
      notify(`${issue.code} · ${issue.title}`);
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
          icon="send"
        />
        <Metric
          label="MASSE SALARIALE"
          value={money(batch.grossTotalCentimes)}
          detail="Brut mensuel"
          icon="calc"
        />
        <Metric
          label="CHARGES EMPLOYEUR"
          value={money(batch.employerChargesTotal)}
          detail="CNSS · AMO · AF · TFP"
          icon="shield"
        />
        <Metric
          label="COLLABORATEURS"
          value={String(period.employeeCount || employees.length)}
          detail={`${employees.length} chargé(s) à l’écran`}
          icon="users"
        />
      </div>
      {route === "cnss" ? (
        <CNSSWorkbench
          period={period}
          employees={employees}
          readiness={cnssReadiness}
          result={cnssResult}
          issue={cnssIssue}
          busy={busy}
          generate={cnss}
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

function AttendancePage({
  employees,
  activePeriod,
  isAdmin,
  notify,
}: {
  employees: Employee[];
  activePeriod: string;
  isAdmin: boolean;
  notify: (value: string) => void;
}) {
  const now = new Date(),
    month = activePeriod || `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`,
    lastDay = new Date(Number(month.slice(0, 4)), Number(month.slice(5, 7)), 0).getDate(),
    monthEnd = `${month}-${String(lastDay).padStart(2, "0")}`,
    [view, setView] = useState<"journal" | "calendar" | "rules">("journal"),
    [journalMode, setJournalMode] = useState<"events" | "days">("events"),
    [filter, setFilter] = useState<PunchFilter>({
      from: `${month}-01`,
      to: monthEnd,
      employeeId: "",
      deviceId: 0,
      query: "",
      cursor: 0,
      limit: 50,
    }),
    [page, setPage] = useState<PunchPage>({ items: [], nextCursor: 0, total: 0, unlinked: 0 }),
    [devices, setDevices] = useState<Device[]>([]),
    [holidays, setHolidays] = useState<Holiday[]>([]),
    [policy, setPolicy] = useState<WorkPolicy>(defaultWorkPolicy),
    [loadingJournal, setLoadingJournal] = useState(true),
    [syncing, setSyncing] = useState(false),
    [editing, setEditing] = useState<Punch | "new" | null>(null),
    [exportOpen, setExportOpen] = useState(false),
    [holidayDraft, setHolidayDraft] = useState<Holiday>({
      date: `${month}-01`,
      name: "",
      kind: "company",
      paid: true,
      premiumBasisPoints: 5000,
      status: "confirmed",
      source: "Décision interne DRH",
    });

  const demoPunches: Punch[] = [
    { id: 3, employeeId: "AT-0018", employeeName: "Lina Benjelloun", deviceId: 1, deviceName: "Entrée principale", deviceUserId: "18", occurredAt: `${month}-10T17:34:00+01:00`, direction: "exit", verification: "face", source: "device", note: "", edited: false, updatedAt: "", unlinked: false },
    { id: 2, employeeId: "AT-0018", employeeName: "Lina Benjelloun", deviceId: 1, deviceName: "Entrée principale", deviceUserId: "18", occurredAt: `${month}-10T08:27:00+01:00`, direction: "entry", verification: "face", source: "device", note: "", edited: false, updatedAt: "", unlinked: false },
    { id: 1, employeeId: "", employeeName: "", deviceId: 1, deviceName: "Entrée principale", deviceUserId: "204", occurredAt: `${month}-09T08:11:00+01:00`, direction: "entry", verification: "fingerprint", source: "device", note: "", edited: false, updatedAt: "", unlinked: true },
  ];

  async function loadJournal(cursor = 0, append = false) {
    setLoadingJournal(true);
    try {
      const request = { ...filter, cursor };
      const next = isNative
        ? ((await ListAttendancePunches(request as never)) as PunchPage)
        : { items: demoPunches, nextCursor: 0, total: 3, unlinked: 1 };
      setPage((current) => ({
        ...next,
        total: next.total < 0 ? current.total : next.total,
        unlinked: next.total < 0 ? current.unlinked : next.unlinked,
        items: append ? [...current.items, ...next.items] : next.items,
      }));
    } catch (error) {
      notify(friendlyError(error));
    } finally {
      setLoadingJournal(false);
    }
  }
  async function loadReferenceData() {
    try {
      if (isNative) {
        const [deviceRows, holidayRows, savedPolicy] = await Promise.all([
          ListAttendanceDevices(),
          ListAttendanceHolidays(month),
          GetAttendanceWorkPolicy(),
        ]);
        setDevices(deviceRows as Device[]);
        setHolidays(holidayRows as Holiday[]);
        setPolicy(savedPolicy as WorkPolicy);
      } else {
        setDevices([{ ...blankDevice, id: 1, lastStatus: "online", linkedUsers: 3 }]);
        setHolidays([
          { date: `${month}-06`, name: "Marche Verte", kind: "national", paid: true, premiumBasisPoints: 10000, status: "confirmed", source: "Calendrier légal intégré; contrôle annuel requis" },
        ]);
      }
    } catch (error) {
      notify(friendlyError(error));
    }
  }
  useEffect(() => {
    loadJournal();
    loadReferenceData();
  }, [month]);

  const dailyRows = useMemo(() => {
    const groups = new Map<string, Punch[]>();
    for (const punch of page.items) {
      const key = `${punch.employeeId || `zk-${punch.deviceUserId}`}|${punch.occurredAt.slice(0, 10)}`;
      groups.set(key, [...(groups.get(key) || []), punch]);
    }
    return [...groups.values()].map((items) => {
      const ordered = [...items].sort((a, b) => a.occurredAt.localeCompare(b.occurredAt));
      let entry: Date | null = null,
        minutes = 0,
        anomalies = 0;
      for (const item of ordered) {
        const direction = item.direction === "unknown" ? (entry ? "exit" : "entry") : item.direction;
        if (direction === "entry") {
          if (entry) anomalies++;
          entry = new Date(item.occurredAt);
        } else if (!entry) anomalies++;
        else {
          minutes += Math.max(0, Math.round((new Date(item.occurredAt).getTime() - entry.getTime()) / 60000));
          entry = null;
        }
      }
      if (entry) anomalies++;
      return { items: ordered, minutes, anomalies };
    });
  }, [page.items]);

  async function synchronize() {
    const targets = devices.filter((device) => device.enabled && device.protocol === "standalone");
    if (!targets.length) {
      notify("Aucune pointeuse Standalone active à synchroniser.");
      return;
    }
    setSyncing(true);
    try {
      let imported = 0,
        ignored = 0,
        unlinked = 0;
      if (isNative) {
        for (const device of targets) {
          const result = await SyncAttendanceDevice(device.id);
          imported += result.imported;
          ignored += result.ignored;
          unlinked += result.unlinked;
        }
      }
      await loadJournal();
      notify(`${imported} nouveau(x) pointage(s) · ${ignored} déjà présent(s) · ${unlinked} à rattacher`);
    } catch (error) {
      notify(friendlyError(error));
    } finally {
      setSyncing(false);
    }
  }

  async function saveHoliday(event: FormEvent) {
    event.preventDefault();
    try {
      if (isNative) await SaveAttendanceHoliday(holidayDraft as never);
      setHolidayDraft({ ...holidayDraft, name: "" });
      await loadReferenceData();
      notify("Jour spécial enregistré avec son coefficient.");
    } catch (error) {
      notify(friendlyError(error));
    }
  }
  async function removeHoliday(date: string) {
    try {
      if (isNative) await DeleteAttendanceHoliday(date);
      await loadReferenceData();
      notify("Jour spécial supprimé.");
    } catch (error) {
      notify(friendlyError(error));
    }
  }
  async function savePolicy(event: FormEvent) {
    event.preventDefault();
    try {
      if (isNative) await SaveAttendanceWorkPolicy(policy as never);
      notify("Règles de temps enregistrées et versionnées.");
    } catch (error) {
      notify(friendlyError(error));
    }
  }

  return (
    <>
      <div className="page-title attendance-title">
        <div>
          <span className="eyebrow">TEMPS → CONTRÔLE → PAIE</span>
          <h1>Journal de pointage</h1>
          <p>Événements bruts, corrections tracées, calendrier et règles horaires dans un même flux.</p>
        </div>
        <div className="page-title-actions">
          {isAdmin && (
            <button className="button secondary" onClick={() => setEditing("new")}>
              <Icon name="plus" /> Pointage manuel
            </button>
          )}
          <button className="button primary" onClick={synchronize} disabled={!isAdmin || syncing}>
            <Icon name="device" /> {syncing ? "Synchronisation…" : "Synchroniser"}
          </button>
        </div>
      </div>

      <div className="attendance-flow" aria-label="Flux de traitement du pointage">
        <span><b>01</b><strong>Collecte</strong><small>Pointeuses toutes les 5 min</small></span>
        <span><b>02</b><strong>Contrôle</strong><small>Liens, doublons, anomalies</small></span>
        <span><b>03</b><strong>Validation</strong><small>Correction avec motif</small></span>
        <span><b>04</b><strong>Paie</strong><small>Impact après approbation</small></span>
      </div>

      <div className="attendance-tabs" role="tablist">
        <button className={view === "journal" ? "active" : ""} onClick={() => setView("journal")}>Journal</button>
        <button className={view === "calendar" ? "active" : ""} onClick={() => setView("calendar")}>Calendrier & coefficients</button>
        <button className={view === "rules" ? "active" : ""} onClick={() => setView("rules")}>Règles de travail</button>
      </div>

      {view === "journal" && (
        <>
          <section className="panel attendance-filters">
            <DateField label="Du" value={filter.from} set={(from) => setFilter({ ...filter, from })} compact />
            <DateField label="Au" value={filter.to} set={(to) => setFilter({ ...filter, to })} compact />
            <SelectField label="Salarié" value={filter.employeeId} set={(employeeId) => setFilter({ ...filter, employeeId })} compact required={false}>
              <option value="">Tous les salariés</option>
              {employees.map((employee) => <option key={employee.id} value={employee.id}>{employee.firstName} {employee.lastName}</option>)}
            </SelectField>
            <SelectField label="Pointeuse" value={String(filter.deviceId)} set={(value) => setFilter({ ...filter, deviceId: Number(value) })} compact required={false}>
              <option value="0">Toutes les pointeuses</option>
              {devices.map((device) => <option key={device.id} value={device.id}>{device.name}</option>)}
            </SelectField>
            <TextField label="Recherche" value={filter.query} set={(query) => setFilter({ ...filter, query })} placeholder="Matricule, nom ou ID ZK" compact required={false} />
            <button className="button secondary" onClick={() => loadJournal()}><Icon name="search" /> Appliquer</button>
          </section>
          <div className="attendance-metrics">
            <span><small>ÉVÉNEMENTS</small><strong>{page.total.toLocaleString("fr")}</strong></span>
            <span><small>NON RATTACHÉS</small><strong>{page.unlinked.toLocaleString("fr")}</strong></span>
            <span><small>CORRIGÉS SUR LA PAGE</small><strong>{page.items.filter((item) => item.edited).length}</strong></span>
            <span><small>SYNC AUTO</small><strong>5 min</strong></span>
          </div>
          {page.unlinked > 0 && (
            <div className="banner attendance-warning"><Icon name="warning" /><div><strong>{page.unlinked} événement(s) attendent un salarié</strong><small>Associez l’ID ZKTeco depuis la page Pointeuses; l’historique déjà reçu sera rattaché automatiquement.</small></div></div>
          )}
          <section className="panel table-panel attendance-table">
            <div className="panel-head">
              <div><h2>{journalMode === "events" ? "Événements horodatés" : "Vue journalière"}</h2><p>{loadingJournal ? "Lecture du coffre local…" : `${page.items.length} ligne(s) chargée(s), mémoire bornée`}</p></div>
              <div className="table-actions">
                <div className="view-switch">
                  <button className={journalMode === "events" ? "active" : ""} onClick={() => setJournalMode("events")}>Événements</button>
                  <button className={journalMode === "days" ? "active" : ""} onClick={() => setJournalMode("days")}>Par jour</button>
                </div>
                <button className="button secondary" onClick={() => setExportOpen(true)} disabled={!isAdmin}><Icon name="send" /> Export personnalisé</button>
              </div>
            </div>
            <div className="table-scroll">
              {journalMode === "events" ? (
                <table>
                  <thead><tr><th>SALARIÉ</th><th>DATE & HEURE</th><th>SENS</th><th>PREUVE</th><th>POINTEUSE</th><th>ÉTAT</th><th /></tr></thead>
                  <tbody>{page.items.map((punch) => (
                    <tr key={punch.id} className={punch.unlinked ? "row-warning" : ""}>
                      <td><span><strong>{punch.employeeName || `ID ZK ${punch.deviceUserId}`}</strong><small>{punch.employeeId || "Non rattaché"}</small></span></td>
                      <td>{new Date(punch.occurredAt).toLocaleString("fr-MA")}</td>
                      <td><em className="badge neutral">{punch.direction === "entry" ? "Entrée" : punch.direction === "exit" ? "Sortie" : "À qualifier"}</em></td>
                      <td>{verificationLabel(punch.verification)}</td>
                      <td>{punch.deviceName || "Saisie manuelle"}</td>
                      <td>{punch.edited ? <em className="badge neutral">Corrigé</em> : punch.source === "device" ? "Brut" : "Manuel"}</td>
                      <td>{isAdmin && <button className="icon-button labeled" onClick={() => setEditing(punch)}><Icon name="more" /> Contrôler</button>}</td>
                    </tr>
                  ))}</tbody>
                </table>
              ) : (
                <table>
                  <thead><tr><th>SALARIÉ</th><th>DATE</th><th>PREMIÈRE ENTRÉE</th><th>DERNIÈRE SORTIE</th><th>TRAVAIL LU</th><th>ANOMALIES</th></tr></thead>
                  <tbody>{dailyRows.map((row) => (
                    <tr key={`${row.items[0].employeeId}-${row.items[0].occurredAt.slice(0, 10)}`}>
                      <td><span><strong>{row.items[0].employeeName || `ID ZK ${row.items[0].deviceUserId}`}</strong><small>{row.items[0].employeeId || "Non rattaché"}</small></span></td>
                      <td>{row.items[0].occurredAt.slice(0, 10)}</td>
                      <td>{row.items.find((item) => item.direction !== "exit")?.occurredAt.slice(11, 16) || "—"}</td>
                      <td>{[...row.items].reverse().find((item) => item.direction === "exit")?.occurredAt.slice(11, 16) || "—"}</td>
                      <td>{formatMinutes(row.minutes)}</td>
                      <td>{row.anomalies ? <em className="badge neutral">{row.anomalies} à revoir</em> : "Contrôlé"}</td>
                    </tr>
                  ))}</tbody>
                </table>
              )}
            </div>
            <div className="panel-foot"><span>Tri du plus récent · données locales SQLite</span>{page.nextCursor > 0 && <button className="button secondary small" onClick={() => loadJournal(page.nextCursor, true)}>Charger la suite</button>}</div>
          </section>
        </>
      )}

      {view === "calendar" && (
        <div className="attendance-calendar-layout">
          <section className="panel holiday-list">
            <div className="panel-head"><div><h2>Calendrier {month}</h2><p>Les dates fixes sont chargées automatiquement; les fêtes religieuses restent à confirmer.</p></div></div>
            {holidays.map((holiday) => (
              <article key={holiday.date}>
                <time>{holiday.date.slice(8, 10)}<small>{new Date(`${holiday.date}T12:00:00`).toLocaleDateString("fr-MA", { month: "short" })}</small></time>
                <span><strong>{holiday.name}</strong><small>{holiday.kind === "company" ? "Jour entreprise" : holiday.kind === "religious" ? "Fête religieuse" : "Fête nationale"} · {holiday.status === "confirmed" ? "Confirmé" : "À confirmer"}</small></span>
                <b>+{holiday.premiumBasisPoints / 100}%</b>
                {isAdmin && holiday.kind === "company" && <button className="icon-button" onClick={() => removeHoliday(holiday.date)} aria-label={`Supprimer ${holiday.name}`}><Icon name="trash" /></button>}
              </article>
            ))}
          </section>
          <form className="panel holiday-form" onSubmit={saveHoliday}>
            <span className="eyebrow">RÈGLE EXCEPTIONNELLE</span><h2>Ajouter un jour spécial</h2>
            <DateField label="Date" value={holidayDraft.date} set={(date) => setHolidayDraft({ ...holidayDraft, date })} />
            <TextField label="Libellé" value={holidayDraft.name} set={(name) => setHolidayDraft({ ...holidayDraft, name })} placeholder="Ex. inventaire annuel" />
            <SelectField label="Type" value={holidayDraft.kind} set={(kind) => setHolidayDraft({ ...holidayDraft, kind: kind as Holiday["kind"] })}>
              <option value="company">Jour spécial entreprise</option><option value="religious">Fête religieuse</option><option value="national">Fête nationale</option>
            </SelectField>
            <TextField label="Majoration horaire (%)" type="number" value={String(holidayDraft.premiumBasisPoints / 100)} set={(value) => setHolidayDraft({ ...holidayDraft, premiumBasisPoints: Math.round(Number(value) * 100) })} min={0} max={500} step={1} />
            <SelectField label="Validation" value={holidayDraft.status} set={(status) => setHolidayDraft({ ...holidayDraft, status: status as Holiday["status"] })}>
              <option value="confirmed">Confirmé</option><option value="provisional">À confirmer</option>
            </SelectField>
            <TextField label="Source / décision" value={holidayDraft.source} set={(source) => setHolidayDraft({ ...holidayDraft, source })} />
            <button className="button primary wide" disabled={!isAdmin}>Enregistrer le jour et son taux</button>
          </form>
        </div>
      )}

      {view === "rules" && (
        <form className="panel attendance-rules" onSubmit={savePolicy}>
          <div className="rules-heading"><div><span className="eyebrow">RÉFÉRENTIEL VERSIONNÉ</span><h2>Temps de travail et majorations</h2><p>Base privée non agricole : 44 h/semaine. Toute proposition vers la paie doit rester contrôlée par l’administrateur.</p></div><em>{policy.rulesVersion}</em></div>
          <div className="form-grid rules-grid">
            <SelectField label="Secteur" value={policy.sector} set={(sector) => setPolicy({ ...policy, sector: sector as WorkPolicy["sector"], weeklyMinutes: sector === "agricultural" ? 2496 * 60 / 52 : 2640 })}>
              <option value="non_agricultural">Non agricole</option><option value="agricultural">Agricole</option>
            </SelectField>
            <TextField label="Durée hebdomadaire (heures)" type="number" value={String(policy.weeklyMinutes / 60)} set={(value) => setPolicy({ ...policy, weeklyMinutes: Math.round(Number(value) * 60) })} min={1} max={168} step={0.25} />
            <TextField label="Début plage de jour" type="time" value={policy.dayStart} set={(dayStart) => setPolicy({ ...policy, dayStart })} />
            <TextField label="Fin plage de jour" type="time" value={policy.dayEnd} set={(dayEnd) => setPolicy({ ...policy, dayEnd })} />
            <TextField label="Pause quotidienne (min)" type="number" value={String(policy.breakMinutes)} set={(value) => setPolicy({ ...policy, breakMinutes: Number(value) })} min={0} max={720} />
            <TextField label="Tolérance retard (min)" type="number" value={String(policy.graceMinutes)} set={(value) => setPolicy({ ...policy, graceMinutes: Number(value) })} min={0} max={180} />
            <TextField label="Majoration jour ouvrable (%)" type="number" value={String(policy.dayPremiumBasisPoints / 100)} set={(value) => setPolicy({ ...policy, dayPremiumBasisPoints: Math.round(Number(value) * 100) })} min={0} />
            <TextField label="Majoration nuit ouvrable (%)" type="number" value={String(policy.nightPremiumBasisPoints / 100)} set={(value) => setPolicy({ ...policy, nightPremiumBasisPoints: Math.round(Number(value) * 100) })} min={0} />
            <TextField label="Repos hebdomadaire de jour (%)" type="number" value={String(policy.restDayPremiumBasisPoints / 100)} set={(value) => setPolicy({ ...policy, restDayPremiumBasisPoints: Math.round(Number(value) * 100) })} min={0} />
            <TextField label="Repos hebdomadaire de nuit (%)" type="number" value={String(policy.restNightPremiumBasisPoints / 100)} set={(value) => setPolicy({ ...policy, restNightPremiumBasisPoints: Math.round(Number(value) * 100) })} min={0} />
          </div>
          <div className="legal-control"><Icon name="shield" /><div><strong>Garde-fou de calcul</strong><small>Les congés, absences justifiées, repos compensateur et conventions collectives peuvent changer le résultat. Le journal calcule une proposition; il ne modifie pas le brut mensuel sans validation dédiée.</small></div></div>
          <div className="dialog-actions"><button className="button primary" disabled={!isAdmin}>Enregistrer les règles</button></div>
        </form>
      )}

      {editing && (
        <AttendancePunchDialog
          value={editing === "new" ? null : editing}
          employees={employees}
          devices={devices}
          close={() => setEditing(null)}
          saved={async () => { setEditing(null); await loadJournal(); }}
          notify={notify}
        />
      )}
      {exportOpen && <AttendanceExportDialog filter={filter} close={() => setExportOpen(false)} notify={notify} />}
    </>
  );
}

function AttendancePunchDialog({
  value,
  employees,
  devices,
  close,
  saved,
  notify,
}: {
  value: Punch | null;
  employees: Employee[];
  devices: Device[];
  close: () => void;
  saved: () => Promise<void>;
  notify: (value: string) => void;
}) {
  const localValue = value?.occurredAt
    ? new Date(value.occurredAt).toISOString().slice(0, 16)
    : new Date().toISOString().slice(0, 16),
    [employeeId, setEmployeeId] = useState(value?.employeeId || ""),
    [deviceId, setDeviceId] = useState(value?.deviceId || 0),
    [occurredAt, setOccurredAt] = useState(localValue),
    [direction, setDirection] = useState(value?.direction || "entry"),
    [verification, setVerification] = useState(value?.verification || "manual"),
    [note, setNote] = useState(value?.note || ""),
    [reason, setReason] = useState(""),
    [history, setHistory] = useState<{ id: number; action: string; reason: string; changedBy: string; changedAt: string }[]>([]),
    [error, setError] = useState("");
  const [occurredDate, occurredTime = ""] = occurredAt.split("T");
  useEffect(() => {
    if (value?.id && isNative) ListAttendancePunchChanges(value.id).then((rows) => setHistory(rows as typeof history)).catch(() => undefined);
  }, [value?.id]);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setError("");
    try {
      const input = {
        id: value?.id || 0,
        employeeId,
        deviceId,
        occurredAt: new Date(occurredAt).toISOString(),
        direction,
        verification,
        note,
        reason,
      };
      if (isNative) await SaveAttendancePunch(input as never);
      await saved();
      notify(value ? "Pointage corrigé; l’ancienne valeur reste dans l’historique." : "Pointage manuel ajouté et journalisé.");
    } catch (caught) {
      setError(friendlyError(caught));
    }
  }
  return (
    <div className="backdrop" onMouseDown={close}>
      <form className="dialog attendance-dialog" onSubmit={submit} onMouseDown={(event) => event.stopPropagation()}>
        <DialogHead eyebrow={value ? "CORRECTION AUDITÉE" : "SAISIE CONTRÔLÉE"} title={value ? "Corriger le pointage" : "Ajouter un pointage"} close={close} />
        <div className="form-grid two">
          <SelectField label="Salarié" value={employeeId} set={setEmployeeId}>
            <option value="">Choisir…</option>{employees.map((employee) => <option key={employee.id} value={employee.id}>{employee.firstName} {employee.lastName} · {employee.id}</option>)}
          </SelectField>
          <div className="date-time-fields">
            <DateField label="Date" value={occurredDate} set={(date) => setOccurredAt(`${date}T${occurredTime || "08:00"}`)} />
            <TextField label="Heure" type="time" value={occurredTime} set={(time) => setOccurredAt(`${occurredDate}T${time}`)} />
          </div>
          <SelectField label="Sens" value={direction} set={(next) => setDirection(next as Punch["direction"])}>
            <option value="entry">Entrée</option><option value="exit">Sortie</option><option value="unknown">À qualifier</option>
          </SelectField>
          <SelectField label="Méthode" value={verification} set={setVerification}>
            <option value="manual">Saisie manuelle</option><option value="face">Visage</option><option value="fingerprint">Empreinte</option><option value="card">Badge</option><option value="password">Code</option><option value="other">Autre</option>
          </SelectField>
          <SelectField label="Pointeuse d’origine" value={String(deviceId)} set={(next) => setDeviceId(Number(next))} required={false}>
            <option value="0">Aucune · saisie manuelle</option>{devices.map((device) => <option key={device.id} value={device.id}>{device.name}</option>)}
          </SelectField>
          <TextField label="Note" value={note} set={setNote} required={false} placeholder="Justificatif ou contexte" />
          {value && <div className="span-two"><TextField label="Motif obligatoire de la correction" value={reason} set={setReason} placeholder="Au moins 5 caractères" minLength={5} /></div>}
        </div>
        {history.length > 0 && <div className="audit-strip"><strong>Historique immuable</strong>{history.map((item) => <span key={item.id}><b>{item.changedBy}</b> · {item.reason} · {new Date(item.changedAt).toLocaleString("fr-MA")}</span>)}</div>}
        {error && <div className="form-error dialog-error"><Icon name="warning" />{error}</div>}
        <div className="dialog-actions"><button type="button" className="button secondary" onClick={close}>Annuler</button><button className="button primary">{value ? "Enregistrer la correction" : "Ajouter au journal"}</button></div>
      </form>
    </div>
  );
}

function AttendanceExportDialog({ filter, close, notify }: { filter: PunchFilter; close: () => void; notify: (value: string) => void }) {
  const options = [
      ["employeeId", "Matricule"], ["employeeName", "Salarié"], ["occurredAt", "Date et heure"],
      ["direction", "Sens"], ["deviceName", "Pointeuse"], ["deviceUserId", "ID pointeuse"],
      ["verification", "Méthode"], ["source", "Source"], ["note", "Note"], ["edited", "Corrigé"],
    ],
    [columns, setColumns] = useState(options.slice(0, 8).map(([key]) => key)),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError("");
    try {
      if (isNative) {
        const result = await SaveAttendanceExport({ filter: { ...filter, cursor: 0 }, columns } as never);
        notify(`${result.recordCount.toLocaleString("fr")} pointage(s) exporté(s) vers ${result.path}`);
      } else notify("Export personnalisé prêt en mode aperçu.");
      close();
    } catch (caught) { setError(friendlyError(caught)); }
    finally { setBusy(false); }
  }
  return (
    <div className="backdrop" onMouseDown={close}>
      <form className="dialog compact export-dialog" onSubmit={submit} onMouseDown={(event) => event.stopPropagation()}>
        <DialogHead eyebrow="EXPORT SUR MESURE" title="Choisir les colonnes" close={close} />
        <p className="dialog-intro">Le filtre actuel est conservé. Le CSV est écrit en flux pour rester rapide même avec un historique massif.</p>
        <div className="column-picker">{options.map(([key, label]) => (
          <label key={key}><input type="checkbox" checked={columns.includes(key)} onChange={(event) => setColumns(event.target.checked ? [...columns, key] : columns.filter((column) => column !== key))} /><span>{label}</span></label>
        ))}</div>
        {error && <div className="form-error dialog-error"><Icon name="warning" />{error}</div>}
        <div className="dialog-actions"><button type="button" className="button secondary" onClick={close}>Annuler</button><button className="button primary" disabled={busy || columns.length === 0}>{busy ? "Écriture…" : "Enregistrer le CSV"}</button></div>
      </form>
    </div>
  );
}

function verificationLabel(value: string) {
  return ({ face: "Visage", fingerprint: "Empreinte", card: "Badge", password: "Code", manual: "Manuel", other: "Autre" } as Record<string, string>)[value] || value;
}

function formatMinutes(value: number) {
  return `${Math.floor(value / 60)} h ${String(value % 60).padStart(2, "0")}`;
}

function DevicesPage({
  employees,
  isAdmin,
  notify,
}: {
  employees: Employee[];
  isAdmin: boolean;
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
      <PointerConnectivityPanel isAdmin={isAdmin} notify={notify} />
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
                  {device.model} · {device.transport === "wireguard" ? "Tunnel privé" : "Réseau local"} · {device.host}:{device.port}
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
            <SelectField
              label="Chemin réseau"
              value={editing.transport}
              set={(value) =>
                setEditing({
                  ...editing,
                  transport: value as Device["transport"],
                })
              }
              hint="Le tunnel reste interne à PaieMaroc"
            >
              <option value="lan">Réseau local direct</option>
              <option value="wireguard">WireGuard espace utilisateur</option>
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
            {editing.protocol === "adms" && (
              <TextField
                label="N° de série ZKTeco"
                value={editing.serialNumber}
                set={(value) =>
                  setEditing({ ...editing, serialNumber: value })
                }
                placeholder="Ex. CLQF24001234"
                hint="Doit correspondre exactement au paramètre SN envoyé par la pointeuse"
                pattern="[A-Za-z0-9._-]{1,64}"
                maxLength={64}
              />
            )}
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

function PointerConnectivityPanel({
  isAdmin,
  notify,
}: {
  isAdmin: boolean;
  notify: (value: string) => void;
}) {
  const emptyConfig: PointerConnectivityConfig = {
    hook: {
      enabled: false,
      bindAddress: "0.0.0.0",
      port: 8088,
      allowedNetworks: ["192.168.0.0/16"],
      firewallConsent: false,
    },
    wireGuard: {
      enabled: false,
      address: "10.77.0.2/32",
      privateKey: "",
      peerPublicKey: "",
      presharedKey: "",
      endpoint: "",
      allowedIPs: ["10.90.0.0/24"],
      persistentKeepaliveSeconds: 25,
      mtu: 1420,
    },
  };
  const [status, setStatus] = useState<PointerConnectivityStatus | null>(null),
    [draft, setDraft] = useState<PointerConnectivityConfig>(emptyConfig),
    [open, setOpen] = useState(false),
    [saving, setSaving] = useState(false),
    [error, setError] = useState(""),
    [generatedPublicKey, setGeneratedPublicKey] = useState(""),
    [importText, setImportText] = useState(""),
    [testTarget, setTestTarget] = useState("10.255.255.34"),
    [testingTunnel, setTestingTunnel] = useState(false),
    [testResult, setTestResult] = useState<PointerConnectivityTest | null>(null);

  const applyStatus = (value: PointerConnectivityStatus) => {
    setStatus(value);
    setDraft({
      hook: { ...value.config.hook },
      wireGuard: {
        enabled: value.config.wireGuard.enabled,
        address: value.config.wireGuard.address || "10.77.0.2/32",
        privateKey: "",
        peerPublicKey: value.config.wireGuard.peerPublicKey || "",
        presharedKey: "",
        endpoint: value.config.wireGuard.endpoint || "",
        allowedIPs: value.config.wireGuard.allowedIPs?.length
          ? value.config.wireGuard.allowedIPs
          : ["10.90.0.0/24"],
        persistentKeepaliveSeconds:
          value.config.wireGuard.persistentKeepaliveSeconds || 25,
        mtu: value.config.wireGuard.mtu || 1420,
      },
    });
  };

  useEffect(() => {
    if (isNative) {
      GetPointerConnectivity()
        .then((value) => applyStatus(value as PointerConnectivityStatus))
        .catch((reason) => setError(friendlyError(reason)));
      return;
    }
    applyStatus({
      collectorActive: false,
      wireGuardActive: false,
      wireGuardHandshake: false,
      wireGuardResolvedEndpoint: "",
      wireGuardLastHandshakeAt: "",
      wireGuardReceivedBytes: 0,
      wireGuardSentBytes: 0,
      lanAddress: "",
      tunnelAddress: "",
      firewallGuidance:
        "Le système demandera l’autorisation des connexions entrantes uniquement pour PUSH / ADMS.",
      lastError: "",
      config: {
        hook: emptyConfig.hook,
        wireGuard: {
          ...emptyConfig.wireGuard,
          hasPrivateKey: false,
          publicKey: "",
          hasPresharedKey: false,
        },
      },
    });
  }, []);

  const networkList = (value: string) =>
    value
      .split(/[;,\n]+/)
      .map((item) => item.trim())
      .filter(Boolean);

  async function generateIdentity() {
    try {
      const identity = isNative
        ? await GeneratePointerWireGuardIdentity()
        : { privateKey: "clé-privée-générée", publicKey: "clé-publique-générée" };
      setDraft((current) => ({
        ...current,
        wireGuard: { ...current.wireGuard, privateKey: identity.privateKey },
      }));
      setGeneratedPublicKey(identity.publicKey);
      notify("Nouvelle identité WireGuard créée. La clé privée restera masquée.");
    } catch (reason) {
      setError(friendlyError(reason));
    }
  }

  function importConfiguration() {
    try {
      const imported = parseWireGuardConfig(importText);
      setDraft((current) => ({
        ...current,
        wireGuard: {
          ...current.wireGuard,
          enabled: true,
          address: imported.address,
          privateKey: imported.privateKey,
          peerPublicKey: imported.peerPublicKey,
          presharedKey: imported.presharedKey,
          endpoint: imported.endpoint,
          allowedIPs: imported.allowedIPs,
          persistentKeepaliveSeconds: imported.persistentKeepaliveSeconds,
          mtu: imported.mtu || current.wireGuard.mtu,
        },
      }));
      setImportText("");
      setError("");
      notify("Configuration importée : clés Interface et Peer placées dans les bons champs.");
    } catch (reason) {
      setError(friendlyError(reason));
    }
  }

  async function testTunnel() {
    setTestingTunnel(true);
    setError("");
    setTestResult(null);
    try {
      const result = isNative
        ? await TestPointerWireGuard(testTarget)
        : {
            target: testTarget,
            reachable: true,
            latencyMilliseconds: 18,
            handshakeEstablished: true,
            lastHandshakeAt: new Date().toISOString(),
            resolvedEndpoint: "172.239.44.171:51820",
            receivedBytes: 128,
            sentBytes: 128,
            message: `Pointeuse ${testTarget} joignable via WireGuard en 18 ms`,
          };
      setTestResult(result as PointerConnectivityTest);
      if (isNative) {
        const nextStatus = await GetPointerConnectivity();
        applyStatus(nextStatus as PointerConnectivityStatus);
      }
    } catch (reason) {
      setError(friendlyError(reason));
    } finally {
      setTestingTunnel(false);
    }
  }

  async function save(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const next = isNative
        ? await SavePointerConnectivity(draft as never)
        : ({
            ...status,
            collectorActive: draft.hook.enabled,
            wireGuardActive: draft.wireGuard.enabled,
            lanAddress: draft.hook.enabled
              ? `${draft.hook.bindAddress}:${draft.hook.port}`
              : "",
            tunnelAddress:
              draft.hook.enabled && draft.wireGuard.enabled
                ? `${draft.wireGuard.address.split("/")[0]}:${draft.hook.port}`
                : "",
            lastError: "",
            config: {
              hook: draft.hook,
              wireGuard: {
                ...draft.wireGuard,
                hasPrivateKey: Boolean(draft.wireGuard.privateKey),
                publicKey: generatedPublicKey,
                hasPresharedKey: Boolean(draft.wireGuard.presharedKey),
              },
            },
          } as PointerConnectivityStatus);
      applyStatus(next as PointerConnectivityStatus);
      setGeneratedPublicKey("");
      notify("Connectivité des pointeuses enregistrée et services rechargés.");
      setOpen(false);
    } catch (reason) {
      setError(friendlyError(reason));
    } finally {
      setSaving(false);
    }
  }

  const localPublicKey = generatedPublicKey || status?.config.wireGuard.publicKey;
  return (
    <section className="panel pointer-connectivity" data-testid="pointer-connectivity">
      <header className="pointer-connectivity-head">
        <div className="pointer-service-mark"><Icon name="shield" size={20} /></div>
        <div>
          <span className="eyebrow">PASSERELLE POINTEUSES</span>
          <h2>Connexions locales et sites distants</h2>
          <p>Services réservés aux terminaux configurés. Aucune route système ni interface noyau n’est créée.</p>
        </div>
        <div className="pointer-service-state">
          <span className={status?.collectorActive ? "active" : "idle"}>
            Collecteur {status?.collectorActive ? "actif" : "arrêté"}
          </span>
          <span className={status?.wireGuardActive ? "active" : "idle"}>
            WireGuard {status?.wireGuardHandshake ? "connecté" : status?.wireGuardActive ? "initialisé" : "arrêté"}
          </span>
        </div>
        <button className="button secondary" onClick={() => setOpen((value) => !value)} disabled={!isAdmin}>
          <Icon name="key" /> {open ? "Fermer" : "Configurer"}
        </button>
      </header>

      <div className="pointer-paths">
        <article>
          <b>01</b><Icon name="device" />
          <span><strong>Standalone</strong><small>PaieMaroc appelle la pointeuse. Aucun port entrant à ouvrir.</small></span>
        </article>
        <article>
          <b>02</b><Icon name="send" />
          <span><strong>PUSH / ADMS</strong><small>La pointeuse envoie son journal au collecteur TCP autorisé.</small></span>
        </article>
        <article>
          <b>03</b><Icon name="lock" />
          <span><strong>Site derrière NAT</strong><small>WireGuard fonctionne dans PaieMaroc, uniquement vers les plages définies.</small></span>
        </article>
      </div>

      {status?.lastError && <div className="pointer-error" role="alert"><Icon name="warning" /><span><strong>Service non démarré</strong><small>{status.lastError}</small></span></div>}
      {error && <div className="pointer-error" role="alert"><Icon name="warning" /><span><strong>Configuration à corriger</strong><small>{error}</small></span></div>}

      {status?.wireGuardActive && <div className="wireguard-diagnostic">
        <div className="wireguard-diagnostic-copy">
          <span className="eyebrow">TEST DU TUNNEL</span>
          <strong>{status.wireGuardHandshake ? "Handshake établi" : "En attente du premier handshake"}</strong>
          <small>Endpoint résolu : {status.wireGuardResolvedEndpoint || "en attente"}</small>
        </div>
        <TextField label="IP de la pointeuse à tester" value={testTarget} set={setTestTarget} placeholder="10.255.255.34" inputMode="decimal" />
        <button type="button" className="button secondary" onClick={testTunnel} disabled={testingTunnel}>
          <Icon name="send" /> {testingTunnel ? "Test…" : "Tester le ping"}
        </button>
        {testResult && <div className={testResult.reachable ? "wireguard-test-result success" : "wireguard-test-result failure"} role="status">
          <strong>{testResult.reachable ? `${testResult.latencyMilliseconds} ms` : testResult.handshakeEstablished ? "Tunnel OK · cible muette" : "Aucun handshake"}</strong>
          <small>{testResult.message}</small>
          <small>{testResult.resolvedEndpoint} · RX {testResult.receivedBytes} o · TX {testResult.sentBytes} o</small>
        </div>}
      </div>}

      {open && (
        <form className="pointer-setup" onSubmit={save}>
          <fieldset>
            <legend><span>01</span><div><strong>Collecteur PUSH / ADMS</strong><small>À activer seulement si une pointeuse initie la connexion.</small></div></legend>
            <label className="pointer-check">
              <input type="checkbox" checked={draft.hook.enabled} onChange={(event) => setDraft({ ...draft, hook: { ...draft.hook, enabled: event.target.checked } })} />
              <span><strong>Recevoir les journaux des pointeuses</strong><small>Écoute locale limitée aux réseaux autorisés ci-dessous.</small></span>
            </label>
            {draft.hook.enabled && <div className="pointer-grid">
              <TextField label="Adresse d’écoute" value={draft.hook.bindAddress} set={(bindAddress) => setDraft({ ...draft, hook: { ...draft.hook, bindAddress } })} hint="0.0.0.0 écoute les interfaces locales" />
              <TextField label="Port ADMS" type="number" value={String(draft.hook.port)} set={(port) => setDraft({ ...draft, hook: { ...draft.hook, port: Number(port) } })} min={1} max={65535} />
              <TextField label="Réseaux de pointeuses autorisés" value={draft.hook.allowedNetworks.join(", ")} set={(value) => setDraft({ ...draft, hook: { ...draft.hook, allowedNetworks: networkList(value) } })} hint="CIDR séparés par des virgules" placeholder="192.168.10.0/24" />
              <div className="firewall-copy"><Icon name="shield" /><span><strong>Autorisation du pare-feu</strong><small>{status?.firewallGuidance}</small></span></div>
              <label className="pointer-check consent">
                <input type="checkbox" checked={draft.hook.firewallConsent} onChange={(event) => setDraft({ ...draft, hook: { ...draft.hook, firewallConsent: event.target.checked } })} required />
                <span><strong>J’autorise cette écoute entrante</strong><small>Je limiterai l’accès au réseau privé des pointeuses.</small></span>
              </label>
            </div>}
          </fieldset>

          <fieldset>
            <legend><span>02</span><div><strong>WireGuard en espace utilisateur</strong><small>Pour joindre une passerelle distante derrière NAT, sans pilote noyau.</small></div></legend>
            <label className="pointer-check">
              <input type="checkbox" checked={draft.wireGuard.enabled} onChange={(event) => setDraft({ ...draft, wireGuard: { ...draft.wireGuard, enabled: event.target.checked } })} />
              <span><strong>Activer le tunnel privé des pointeuses</strong><small>Seules les fiches marquées « WireGuard » l’utiliseront.</small></span>
            </label>
            {draft.wireGuard.enabled && <>
              <div className="wireguard-import">
                <div><strong>Importer un fichier WireGuard</strong><small>Collez le contenu complet. DNS est ignoré car le tunnel ne sert qu’aux pointeuses.</small></div>
                <textarea aria-label="Configuration WireGuard à importer" value={importText} onChange={(event) => setImportText(event.target.value)} placeholder={'[Interface]\nPrivateKey = …\nAddress = 10.255.255.36/32\n\n[Peer]\nPublicKey = …\nEndpoint = wg.example.ma:51820\nAllowedIPs = 10.255.255.32/27'} spellCheck={false} />
                <button type="button" className="button secondary" onClick={importConfiguration} disabled={!importText.trim()}>Analyser et remplir</button>
              </div>
              <div className="pointer-grid wireguard-grid">
                <TextField label="Adresse privée PaieMaroc" value={draft.wireGuard.address} set={(address) => setDraft({ ...draft, wireGuard: { ...draft.wireGuard, address } })} placeholder="10.77.0.2/32" />
                <TextField label="[Interface] PrivateKey" type="password" value={draft.wireGuard.privateKey || ""} set={(privateKey) => setDraft({ ...draft, wireGuard: { ...draft.wireGuard, privateKey } })} required={!status?.config.wireGuard.hasPrivateKey} autoComplete="new-password" hint={status?.config.wireGuard.hasPrivateKey ? "Laisser vide pour conserver la clé protégée" : "Clé privée de cette application"} />
                <TextField label="[Peer] PublicKey" value={draft.wireGuard.peerPublicKey} set={(peerPublicKey) => setDraft({ ...draft, wireGuard: { ...draft.wireGuard, peerPublicKey } })} hint="Clé publique de la passerelle distante" />
                <TextField label="Passerelle WireGuard · [Peer] Endpoint" value={draft.wireGuard.endpoint} set={(endpoint) => setDraft({ ...draft, wireGuard: { ...draft.wireGuard, endpoint } })} placeholder="vpn.societe.ma:51820" hint="Nom DNS ou adresse IP — résolution automatique au démarrage" />
                <TextField label="Sous-réseaux distants des pointeuses" value={draft.wireGuard.allowedIPs.join(", ")} set={(value) => setDraft({ ...draft, wireGuard: { ...draft.wireGuard, allowedIPs: networkList(value) } })} hint="Les routes par défaut sont refusées" placeholder="10.90.0.0/24" />
                <TextField label="Clé prépartagée (optionnelle)" type="password" value={draft.wireGuard.presharedKey || ""} set={(presharedKey) => setDraft({ ...draft, wireGuard: { ...draft.wireGuard, presharedKey } })} required={false} autoComplete="new-password" />
              </div>
              <div className="wireguard-identity">
                <div><small>CLÉ PUBLIQUE LOCALE À COPIER SUR LA PASSERELLE</small><strong>{localPublicKey || "Générez d’abord l’identité PaieMaroc"}</strong></div>
                <button type="button" className="button secondary" onClick={generateIdentity}>Générer l’identité</button>
              </div>
              <div className="pointer-grid compact-network-grid">
                <TextField label="Maintien NAT (secondes)" type="number" value={String(draft.wireGuard.persistentKeepaliveSeconds)} set={(value) => setDraft({ ...draft, wireGuard: { ...draft.wireGuard, persistentKeepaliveSeconds: Number(value) } })} min={0} max={3600} hint="25 s recommandé derrière NAT" />
                <TextField label="MTU" type="number" value={String(draft.wireGuard.mtu)} set={(value) => setDraft({ ...draft, wireGuard: { ...draft.wireGuard, mtu: Number(value) } })} min={1280} max={9000} />
              </div>
            </>}
          </fieldset>
          <div className="pointer-setup-actions">
            <span><Icon name="lock" /><small>La clé privée n’est jamais renvoyée à l’interface après enregistrement.</small></span>
            <button className="button primary" disabled={saving}>{saving ? "Démarrage…" : "Enregistrer et appliquer"}<Icon name="arrow" /></button>
          </div>
        </form>
      )}
    </section>
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
            hint="9 chiffres · vide si nouvel entrant"
            value={employee.cnss}
            set={(cnss) =>
              update({ ...employee, cnss: cnss.replace(/[^0-9]/g, "") }, "cnss")
            }
            error={errors.cnss}
            pattern={employeePatterns.cnss}
            maxLength={9}
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
        <MonthField label="Période" value={period} set={setPeriod} />
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
  icon,
}: {
  label: string;
  value: string;
  detail: string;
  icon: string;
}) {
  return (
    <article className="panel metric">
      <div>
        <span>{label}</span>
        <i className="metric-icon" aria-hidden="true">
          <Icon name={icon} size={16} />
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
  minLength,
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
  minLength?: number;
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
          minLength={minLength}
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
function CNSSWorkbench({
  period,
  employees,
  readiness,
  result,
  issue,
  busy,
  generate,
}: {
  period: Period;
  employees: Employee[];
  readiness: CNSSReadiness;
  result: CNSSResult | null;
  issue: CNSSErrorNotice | null;
  busy: boolean;
  generate: () => void;
}) {
  const completed = Boolean(result);
  return (
    <section className="panel bds-workbench" data-testid="cnss-workbench">
      <header className="bds-head">
        <div>
          <span className="eyebrow">ASSISTANT E-BDS · {period.period}</span>
          <h2>Contrôler avant de remettre</h2>
          <p>
            Un seul fichier à choisir : le préétabli CNSS du mois. Les données
            de paie viennent de la période enregistrée.
          </p>
        </div>
        <span
          className={`bds-status ${readiness.canGenerate ? "ready" : "blocked"}`}
        >
          <Icon name={readiness.canGenerate ? "check" : "warning"} size={14} />
          {readiness.canGenerate ? "Prêt à contrôler" : "Correction requise"}
        </span>
      </header>

      <div className="bds-layout">
        <ol className="bds-steps" aria-label="Étapes de génération BDS">
          <li className={readiness.canGenerate ? "complete" : "blocked"}>
            <b>01</b>
            <span>
              <strong>Période enregistrée</strong>
              <small>
                {period.employeeCount.toLocaleString("fr")} salarié(s) · données
                SQLite du mois
              </small>
            </span>
            <em>{readiness.canGenerate ? "PRÊT" : "À CORRIGER"}</em>
          </li>
          <li className={busy || completed ? "active" : "waiting"}>
            <b>02</b>
            <span>
              <strong>Préétabli CNSS</strong>
              <small>
                Contrôle A00–A03, période et ordre des immatriculations
              </small>
            </span>
            <em>{busy ? "LECTURE" : completed ? "VALIDÉ" : "À CHOISIR"}</em>
          </li>
          <li className={completed ? "complete" : "waiting"}>
            <b>03</b>
            <span>
              <strong>Fichier BDS</strong>
              <small>Assemblage B00–B06 et contrôle des totaux</small>
            </span>
            <em>{completed ? "CRÉÉ" : "EN ATTENTE"}</em>
          </li>
        </ol>

        <div className="bds-control">
          <div className="bds-summary">
            <span>
              <small>SALARIÉS DU MOIS</small>
              <strong>{period.employeeCount.toLocaleString("fr")}</strong>
            </span>
            <span>
              <small>ENTRANTS REPÉRÉS</small>
              <strong>{readiness.entrantCount.toLocaleString("fr")}</strong>
            </span>
            <span>
              <small>BLOCAGES</small>
              <strong>{readiness.errors.length}</strong>
            </span>
          </div>

          {readiness.errors.map((notice) => (
            <BDSNotice key={notice.code} tone="error" notice={notice} />
          ))}
          {readiness.warnings.map((notice) => (
            <BDSNotice key={notice.code} tone="warning" notice={notice} />
          ))}
          {issue && (
            <BDSNotice
              tone={issue.cancelled ? "info" : "error"}
              notice={issue}
              role={issue.cancelled ? "status" : "alert"}
            />
          )}
          {result && (
            <div className="bds-result" role="status">
              <Icon name="check" size={20} />
              <div>
                <span className="eyebrow">BDS GÉNÉRÉ ET CONTRÔLÉ</span>
                <strong>
                  {result.employeeCount.toLocaleString("fr")} salariés ·{" "}
                  {result.entrantCount.toLocaleString("fr")} entrant(s)
                </strong>
                <small>{result.path}</small>
              </div>
              <b>{money(result.grossCentimes)}</b>
            </div>
          )}

          <div className="bds-action">
            <div>
              <strong>Le fichier source reste intact</strong>
              <small>
                En cas d’erreur, PaieMaroc supprime automatiquement la sortie
                incomplète.
              </small>
            </div>
            <button
              className="button primary"
              onClick={generate}
              disabled={busy || !readiness.canGenerate}
            >
              {busy
                ? "Contrôle en cours…"
                : completed
                  ? "Générer à nouveau"
                  : "Choisir et générer"}
              <Icon name="arrow" />
            </button>
          </div>
        </div>
      </div>
    </section>
  );
}

function BDSNotice({
  notice,
  tone,
  role,
}: {
  notice: {
    code: string;
    title: string;
    detail: string;
    employeeIDs?: string[];
  };
  tone: "error" | "warning" | "info";
  role?: "alert" | "status";
}) {
  return (
    <div className={`bds-notice ${tone}`} role={role}>
      <span>
        <Icon
          name={
            tone === "error"
              ? "warning"
              : tone === "warning"
                ? "clock"
                : "shield"
          }
          size={16}
        />
      </span>
      <div>
        <small>{notice.code}</small>
        <strong>{notice.title}</strong>
        <p>{notice.detail}</p>
        {notice.employeeIDs?.length ? (
          <em>Fiches : {notice.employeeIDs.join(", ")}</em>
        ) : null}
      </div>
    </div>
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
