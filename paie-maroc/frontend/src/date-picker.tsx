import {useEffect, useId, useMemo, useRef, useState} from "react";

type PickerProps = {
  label: string;
  value: string;
  set: (value: string) => void;
  compact?: boolean;
  required?: boolean;
  min?: string;
  max?: string;
};

const monthNames = [
  "janvier", "février", "mars", "avril", "mai", "juin",
  "juillet", "août", "septembre", "octobre", "novembre", "décembre",
];
const monthShort = ["janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."];
const weekDays = ["L", "M", "M", "J", "V", "S", "D"];

function pad(value: number): string {
  return String(value).padStart(2, "0");
}

function localISO(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

function dateFromISO(value: string): Date | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) return null;
  const year = Number(match[1]);
  const month = Number(match[2]);
  const day = Number(match[3]);
  const date = new Date(year, month - 1, day, 12);
  return date.getFullYear() === year && date.getMonth() === month - 1 && date.getDate() === day
    ? date
    : null;
}

export function formatDateInput(value: string): string {
  const date = dateFromISO(value);
  return date ? `${pad(date.getDate())}/${pad(date.getMonth() + 1)}/${date.getFullYear()}` : "";
}

export function parseDateInput(value: string): string | null {
  const trimmed = value.trim();
  if (/^\d{4}-\d{2}-\d{2}$/.test(trimmed)) {
    return dateFromISO(trimmed) ? trimmed : null;
  }
  const digits = trimmed.replace(/\D/g, "");
  if (digits.length !== 8) return null;
  const iso = `${digits.slice(4, 8)}-${digits.slice(2, 4)}-${digits.slice(0, 2)}`;
  return dateFromISO(iso) ? iso : null;
}

export function parseMonthInput(value: string): string | null {
  const trimmed = value.trim();
  const isoMatch = /^(\d{4})-(\d{2})$/.exec(trimmed);
  if (isoMatch) {
    const month = Number(isoMatch[2]);
    return month >= 1 && month <= 12 ? trimmed : null;
  }
  const digits = trimmed.replace(/\D/g, "");
  if (digits.length !== 6) return null;
  const month = Number(digits.slice(0, 2));
  return month >= 1 && month <= 12 ? `${digits.slice(2, 6)}-${digits.slice(0, 2)}` : null;
}

function dateDraft(value: string): string {
  const digits = value.replace(/\D/g, "").slice(0, 8);
  return [digits.slice(0, 2), digits.slice(2, 4), digits.slice(4, 8)]
    .filter(Boolean)
    .join("/");
}

function inRange(value: string, min?: string, max?: string): boolean {
  return (!min || value >= min) && (!max || value <= max);
}

function CalendarIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <rect x="3" y="5" width="18" height="16" rx="2" />
      <path d="M8 3v4M16 3v4M3 10h18" />
    </svg>
  );
}

export function DateField({label, value, set, compact = false, required = true, min, max}: PickerProps) {
  const id = useId();
  const root = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const selected = dateFromISO(value);
  const [draft, setDraft] = useState(formatDateInput(value));
  const [open, setOpen] = useState(false);
  const [invalid, setInvalid] = useState(false);
  const [view, setView] = useState(() => selected || new Date());

  useEffect(() => {
    setDraft(formatDateInput(value));
    const next = dateFromISO(value);
    if (next) setView(next);
  }, [value]);

  useEffect(() => {
    if (!open) return;
    const close = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, [open]);

  const days = useMemo(() => {
    const first = new Date(view.getFullYear(), view.getMonth(), 1, 12);
    const mondayOffset = (first.getDay() + 6) % 7;
    return Array.from({length: 42}, (_, index) =>
      new Date(view.getFullYear(), view.getMonth(), index - mondayOffset + 1, 12),
    );
  }, [view]);

  function commit(nextDraft: string): boolean {
    if (!nextDraft.trim() && !required) {
      input.current?.setCustomValidity("");
      setInvalid(false);
      set("");
      return true;
    }
    const parsed = parseDateInput(nextDraft);
    const valid = Boolean(parsed && inRange(parsed, min, max));
    input.current?.setCustomValidity(valid ? "" : "Saisissez une date valide.");
    setInvalid(!valid);
    if (parsed && valid) set(parsed);
    return valid;
  }

  function choose(next: string) {
    set(next);
    setDraft(formatDateInput(next));
    setInvalid(false);
    input.current?.setCustomValidity("");
    setOpen(false);
    input.current?.focus();
  }

  return (
    <div ref={root} className={`field pm-control date-field ${compact ? "compact-control" : ""} ${invalid ? "invalid" : ""} ${open ? "is-open" : ""}`}>
      <label className="control-label" htmlFor={id}>{label}<small>JJ/MM/AAAA</small></label>
      <span className="control-shell date-control-shell">
        <input
          ref={input}
          id={id}
          value={draft}
          onChange={(event) => {
            const raw = event.target.value;
            const parsedISO = /^\d{4}-\d{2}-\d{2}$/.test(raw) ? parseDateInput(raw) : null;
            const next = parsedISO ? formatDateInput(parsedISO) : dateDraft(raw);
            setDraft(next);
            setInvalid(false);
            event.currentTarget.setCustomValidity("");
            if (next.length === 10) commit(next);
          }}
          onBlur={() => commit(draft)}
          onKeyDown={(event) => {
            if (event.key === "ArrowDown") {
              event.preventDefault();
              setOpen(true);
            } else if (event.key === "Escape") {
              setOpen(false);
            } else if (event.key === "Enter" && commit(draft)) {
              setOpen(false);
            }
          }}
          inputMode="numeric"
          autoComplete="off"
          placeholder="JJ/MM/AAAA"
          maxLength={10}
          required={required}
          aria-invalid={invalid}
          aria-describedby={invalid ? `${id}-error` : undefined}
        />
        <button type="button" className="date-trigger" onClick={() => setOpen((current) => !current)} aria-label={`Ouvrir le calendrier — ${label}`} aria-expanded={open}>
          <CalendarIcon />
        </button>
      </span>
      {invalid && <small className="field-error" id={`${id}-error`}>Date invalide ou hors période autorisée.</small>}
      {open && (
        <div className="pm-calendar" role="dialog" aria-label={`Calendrier — ${label}`}>
          <header>
            <button type="button" onClick={() => setView(new Date(view.getFullYear(), view.getMonth() - 1, 1, 12))} aria-label="Mois précédent">‹</button>
            <strong>{monthNames[view.getMonth()]} <b>{view.getFullYear()}</b></strong>
            <button type="button" onClick={() => setView(new Date(view.getFullYear(), view.getMonth() + 1, 1, 12))} aria-label="Mois suivant">›</button>
          </header>
          <div className="pm-calendar-weekdays" aria-hidden="true">{weekDays.map((day, index) => <span key={`${day}-${index}`}>{day}</span>)}</div>
          <div className="pm-calendar-days">
            {days.map((day) => {
              const iso = localISO(day);
              const outside = day.getMonth() !== view.getMonth();
              return (
                <button
                  type="button"
                  key={iso}
                  className={`${outside ? "outside" : ""} ${iso === value ? "selected" : ""} ${iso === localISO(new Date()) ? "today" : ""}`}
                  onClick={() => choose(iso)}
                  disabled={!inRange(iso, min, max)}
                  aria-label={day.toLocaleDateString("fr-MA", {dateStyle: "long"})}
                  aria-pressed={iso === value}
                >{day.getDate()}</button>
              );
            })}
          </div>
          <footer>
            {!required && <button type="button" onClick={() => choose("")}>Effacer</button>}
            <button type="button" onClick={() => choose(localISO(new Date()))}>Aujourd’hui</button>
          </footer>
        </div>
      )}
    </div>
  );
}

export function MonthField({label, value, set, required = true}: Omit<PickerProps, "compact" | "min" | "max">) {
  const id = useId();
  const root = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const currentYear = /^\d{4}-\d{2}$/.test(value) ? Number(value.slice(0, 4)) : new Date().getFullYear();
  const [year, setYear] = useState(currentYear);
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(value ? `${value.slice(5, 7)}/${value.slice(0, 4)}` : "");
  const [invalid, setInvalid] = useState(false);
  useEffect(() => {
    setDraft(value ? `${value.slice(5, 7)}/${value.slice(0, 4)}` : "");
    if (/^\d{4}-\d{2}$/.test(value)) setYear(Number(value.slice(0, 4)));
  }, [value]);
  useEffect(() => {
    if (!open) return;
    const close = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, [open]);
  return (
    <div ref={root} className={`field pm-control date-field month-field ${invalid ? "invalid" : ""} ${open ? "is-open" : ""}`}>
      <label className="control-label" htmlFor={id}>{label}<small>MM/AAAA</small></label>
      <span className="control-shell date-control-shell">
        <input
          ref={input}
          id={id}
          value={draft}
          required={required}
          inputMode="numeric"
          autoComplete="off"
          placeholder="MM/AAAA"
          maxLength={7}
          aria-invalid={invalid}
          onChange={(event) => {
            const raw = event.target.value;
            const parsedISO = /^\d{4}-\d{2}$/.test(raw) ? parseMonthInput(raw) : null;
            const digits = raw.replace(/\D/g, "").slice(0, 6);
            const next = parsedISO ? `${parsedISO.slice(5, 7)}/${parsedISO.slice(0, 4)}` : [digits.slice(0, 2), digits.slice(2, 6)].filter(Boolean).join("/");
            setDraft(next);
            setInvalid(false);
            event.currentTarget.setCustomValidity("");
            const parsed = parseMonthInput(next);
            if (parsed) set(parsed);
          }}
          onBlur={() => {
            const parsed = parseMonthInput(draft);
            const valid = Boolean(parsed) || (!required && !draft);
            input.current?.setCustomValidity(valid ? "" : "Saisissez un mois valide.");
            setInvalid(!valid);
            if (parsed) set(parsed);
          }}
        />
        <button type="button" className="date-trigger" onClick={() => setOpen((current) => !current)} aria-label={`Choisir le mois — ${label}`} aria-expanded={open}><CalendarIcon /></button>
      </span>
      {invalid && <small className="field-error">Mois invalide.</small>}
      {open && (
        <div className="pm-calendar pm-month-picker" role="dialog" aria-label={`Mois — ${label}`}>
          <header>
            <button type="button" onClick={() => setYear((current) => current - 1)} aria-label="Année précédente">‹</button>
            <strong><b>{year}</b></strong>
            <button type="button" onClick={() => setYear((current) => current + 1)} aria-label="Année suivante">›</button>
          </header>
          <div className="pm-month-grid">
            {monthNames.map((month, index) => {
              const next = `${year}-${pad(index + 1)}`;
              return <button type="button" key={month} className={next === value ? "selected" : ""} onClick={() => { set(next); setInvalid(false); setOpen(false); }}>{monthShort[index]}</button>;
            })}
          </div>
          <footer><button type="button" onClick={() => { const today = new Date(); set(`${today.getFullYear()}-${pad(today.getMonth() + 1)}`); setOpen(false); }}>Mois actuel</button></footer>
        </div>
      )}
    </div>
  );
}
