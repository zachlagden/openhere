import { useState, type FormEvent, type JSX } from "react";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/field";
import { useToast } from "@/components/ui/toast";
import { FormError } from "@/features/auth/form-error";
import { cleanDomainInput } from "@/lib/domains";
import { useCreateDomain } from "@/queries/site-detail";

export function AddDomainForm({ slug, disabled }: { slug: string; disabled: boolean }): JSX.Element {
  const create = useCreateDomain(slug);
  const toast = useToast();
  const [text, setText] = useState("");

  function submit(event: FormEvent<HTMLFormElement>): void {
    event.preventDefault();
    create.mutate(cleanDomainInput(text), {
      onSuccess: (domain) => {
        setText("");
        toast.success(`Added ${domain.domain}. Add the TXT record to verify it.`);
      },
    });
  }

  return (
    <form onSubmit={submit} className="flex flex-col gap-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start">
        <div className="sm:w-96">
          <TextField
            label="Domain name"
            value={text}
            onChange={(event) => setText(event.target.value)}
            placeholder="docs.example.org"
            hint="A bare name without https:// or a path."
            autoComplete="off"
            spellCheck={false}
            disabled={disabled}
            required
          />
        </div>
        <Button type="submit" variant="primary" className="sm:mt-6" disabled={disabled || create.isPending || text.trim() === ""}>
          {create.isPending ? "Adding…" : "Add domain"}
        </Button>
      </div>
      <FormError error={create.error} />
    </form>
  );
}
