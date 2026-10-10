import type { JSX, ReactNode } from "react";
import { Alert } from "./alert";
import { Button } from "./button";
import { CopyField } from "./copy-field";
import { Dialog, DialogFooter } from "./dialog";

interface SecretDialogProps {
  title: string;
  fieldLabel: string;
  secret: string | null;
  warning: ReactNode;
  onClose: () => void;
  children?: ReactNode;
}

export function SecretDialog({ title, fieldLabel, secret, warning, onClose, children }: SecretDialogProps): JSX.Element {
  return (
    <Dialog
      open={secret !== null}
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
      title={title}
    >
      {secret !== null ? (
        <div className="flex flex-col gap-4">
          <Alert tone="warning">{warning}</Alert>
          <CopyField label={fieldLabel} value={secret} testId="one-time-secret" />
          {children}
          <DialogFooter>
            <Button variant="primary" autoFocus onClick={onClose}>
              I have saved it
            </Button>
          </DialogFooter>
        </div>
      ) : null}
    </Dialog>
  );
}
