import { useState, type JSX, type ReactNode } from "react";
import { describeError } from "@/api/errors";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { Input, Label, Select, SelectField, TextField } from "@/components/ui/field";
import { SwitchField } from "@/components/ui/switch";
import { useToast } from "@/components/ui/toast";
import { PasswordField } from "@/features/auth/password-field";
import { splitBytes, toBytes, type ByteUnit } from "@/lib/units";
import { buildClearPatch, fieldError, type SettingKey } from "@/lib/settings-form";
import { useUpdateSettings } from "@/queries/admin";
import { useSettingsFormApi } from "./settings-form";

interface FieldProps {
  name: SettingKey;
  label: string;
  hint?: string;
  disabled?: boolean;
}

export function SettingSwitch({ name, label, hint, disabled, description }: FieldProps & { description?: ReactNode }): JSX.Element {
  const { draft, set, failure } = useSettingsFormApi();
  const error = fieldError(failure, name);
  return (
    <div className="flex flex-col gap-1">
      <SwitchField label={label} description={description ?? hint} checked={draft[name] === true} disabled={disabled} onChange={(event) => set(name, event.target.checked)} />
      {error ? (
        <p role="alert" className="text-xs text-danger">
          {error}
        </p>
      ) : null}
    </div>
  );
}

export function SettingText({ name, label, hint, disabled, placeholder, type = "text" }: FieldProps & { placeholder?: string; type?: "text" | "email" }): JSX.Element {
  const { draft, set, failure } = useSettingsFormApi();
  const value = draft[name];
  return (
    <TextField
      label={label}
      hint={hint}
      type={type}
      value={typeof value === "string" ? value : ""}
      placeholder={placeholder}
      disabled={disabled}
      error={fieldError(failure, name)}
      autoComplete="off"
      spellCheck={false}
      onChange={(event) => set(name, event.target.value)}
    />
  );
}

export function SettingSelect({ name, label, hint, disabled, options }: FieldProps & { options: { value: string; label: string }[] }): JSX.Element {
  const { draft, set, failure } = useSettingsFormApi();
  const value = draft[name];
  return (
    <SelectField label={label} hint={hint} disabled={disabled} error={fieldError(failure, name)} value={typeof value === "string" ? value : ""} onChange={(event) => set(name, event.target.value)}>
      {options.map((option) => (
        <option key={option.value} value={option.value}>
          {option.label}
        </option>
      ))}
    </SelectField>
  );
}

export function SettingNumber({ name, label, hint, disabled, min, max }: FieldProps & { min?: number; max?: number }): JSX.Element {
  const { draft, set, failure } = useSettingsFormApi();
  const value = draft[name];
  const [text, setText] = useState(typeof value === "number" ? String(value) : "");
  const invalid = text.trim() === "" || !Number.isInteger(Number(text));
  const range = min !== undefined && max !== undefined ? ` Use ${min} to ${max}.` : "";
  return (
    <TextField
      label={label}
      hint={`${hint ?? ""}${range}`.trim() || undefined}
      type="number"
      inputMode="numeric"
      min={min}
      max={max}
      step={1}
      value={text}
      disabled={disabled}
      error={fieldError(failure, name) ?? (invalid ? "Enter a whole number." : null)}
      onChange={(event) => {
        setText(event.target.value);
        const parsed = Number(event.target.value);
        set(name, event.target.value.trim() !== "" && Number.isInteger(parsed) ? parsed : Number.NaN);
      }}
    />
  );
}

const UNITS: ByteUnit[] = ["MB", "GB"];

export function SettingBytes({ name, label, hint, disabled }: FieldProps): JSX.Element {
  const { draft, set, failure } = useSettingsFormApi();
  const value = draft[name];
  const initial = splitBytes(typeof value === "number" ? value : 0);
  const [amount, setAmount] = useState(String(initial.amount));
  const [unit, setUnit] = useState<ByteUnit>(initial.unit);
  const id = `bytes-${name}`;
  const error = fieldError(failure, name);

  function commit(nextAmount: string, nextUnit: ByteUnit): void {
    const parsed = Number(nextAmount);
    set(name, nextAmount.trim() !== "" && Number.isFinite(parsed) && parsed > 0 ? toBytes(parsed, nextUnit) : Number.NaN);
  }

  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <div className="flex gap-2">
        <Input id={id} type="number" inputMode="decimal" min={0} step="any" value={amount} disabled={disabled} className="w-32" onChange={(event) => { setAmount(event.target.value); commit(event.target.value, unit); }} />
        <Select aria-label={`${label} unit`} value={unit} disabled={disabled} className="w-24" onChange={(event) => { const next = event.target.value === "GB" ? "GB" : "MB"; setUnit(next); commit(amount, next); }}>
          {UNITS.map((item) => (
            <option key={item} value={item}>
              {item}
            </option>
          ))}
        </Select>
      </div>
      {hint ? <p className="text-xs text-muted">{hint}</p> : null}
      {error ? (
        <p role="alert" className="text-xs text-danger">
          {error}
        </p>
      ) : null}
    </div>
  );
}

interface SecretFieldProps extends FieldProps {
  setFlag: SettingKey;
}

export function SettingSecret({ name, setFlag, label, hint, disabled }: SecretFieldProps): JSX.Element {
  const { settings, secrets, setSecret, failure } = useSettingsFormApi();
  const update = useUpdateSettings();
  const toast = useToast();
  const [confirming, setConfirming] = useState(false);
  const isSet = settings[setFlag] === true;

  function clear(): void {
    update.mutate(buildClearPatch(name), {
      onSuccess: () => {
        setConfirming(false);
        toast.success(`${label} cleared.`);
      },
    });
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-sm font-medium">{label}</span>
        <Badge tone={isSet ? "ok" : "neutral"}>{isSet ? "Set" : "Not set"}</Badge>
      </div>
      <div className="max-w-sm">
        <PasswordField
          label={isSet ? "Replace with a new value" : "Value"}
          value={secrets[name] ?? ""}
          onChange={(event) => setSecret(name, event.target.value)}
          autoComplete="new-password"
          disabled={disabled}
          hint={hint ?? "Write-only. The server stores it encrypted and never shows it again."}
        />
      </div>
      {fieldError(failure, name) ? (
        <p role="alert" className="text-xs text-danger">
          {fieldError(failure, name)}
        </p>
      ) : null}
      {isSet ? (
        <div>
          <Button variant="secondary" size="sm" disabled={disabled} onClick={() => setConfirming(true)}>
            Clear {label.toLowerCase()}
          </Button>
        </div>
      ) : null}
      <ConfirmDialog
        open={confirming}
        onOpenChange={(open) => {
          if (!open) {
            update.reset();
          }
          setConfirming(open);
        }}
        title={`Clear ${label.toLowerCase()}?`}
        confirmLabel="Clear value"
        pending={update.isPending}
        error={update.error ? describeError(update.error).message : null}
        onConfirm={clear}
      >
        <p>The stored value is deleted. Anything that needs it stops working until you set a new one.</p>
      </ConfirmDialog>
    </div>
  );
}
