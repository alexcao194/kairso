"use client";

import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { deleteUsersMeApiKey, getUsersMeApiKeys, postUsersMeApiKeys } from "@/lib/api/endpoints";
import type { CreatedAPIKeyResponse } from "@/lib/api/schema";
import { qk } from "@/lib/query/keys";
import { fmtDate, fmtRelative } from "@/lib/domain/format";
import { useT } from "@/lib/i18n";
import { useToast } from "@/lib/toast/ToastProvider";
import { Button } from "@/components/ui/Button";
import { Alert, EmptyState, ErrorState, SkeletonRows } from "@/components/ui/feedback";
import { Field, Input, Select } from "@/components/ui/form";
import { Modal } from "@/components/ui/Modal";
import { useConfirm } from "@/components/ui/Confirm";
import { IconCopy, IconPlus, IconSettings, IconTrash } from "@/components/icons";

const EXPIRY_OPTIONS = [0, 30, 90, 365];

/** The public MCP URL: the web app's own /api/mcp route, which forwards to the API server. */
function useMcpUrl() {
  const [url, setUrl] = useState("https://<kairso>/api/mcp");
  useEffect(() => setUrl(`${window.location.origin}/api/mcp`), []);
  return url;
}

function CopyBlock({ text, label }: { text: string; label: string }) {
  const toast = useToast();
  const t = useT().apiKeys;
  return (
    <div className="relative">
      <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-md bg-surface-sunken p-3 pr-24 font-mono text-[11.5px] leading-relaxed text-ink-2">{text}</pre>
      <Button
        size="sm"
        variant="ghost"
        aria-label={label}
        className="absolute right-1.5 top-1.5"
        onClick={async () => {
          try {
            await navigator.clipboard.writeText(text);
            toast.success(t.copied);
          } catch {
            window.prompt(label, text);
          }
        }}
      >
        <IconCopy size={13} /> {t.copy}
      </Button>
    </div>
  );
}

export function ApiKeysSection() {
  const tAll = useT();
  const t = tAll.apiKeys;
  const toast = useToast();
  const confirm = useConfirm();
  const qc = useQueryClient();
  const mcpUrl = useMcpUrl();
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<CreatedAPIKeyResponse | null>(null);

  const q = useQuery({ queryKey: qk.apiKeys(), queryFn: () => getUsersMeApiKeys() });
  const revoke = useMutation({
    mutationFn: (id: string) => deleteUsersMeApiKey(id),
    onSuccess: () => {
      toast.success(t.revoked);
      qc.invalidateQueries({ queryKey: qk.apiKeys() });
    },
    onError: (e) => toast.error(e),
  });

  const claudeCmd = (key: string) => `claude mcp add --transport http kairso ${mcpUrl} --header "Authorization: Bearer ${key}"`;
  const jsonConfig = (key: string) =>
    JSON.stringify({ mcpServers: { kairso: { type: "http", url: mcpUrl, headers: { Authorization: `Bearer ${key}` } } } }, null, 2);

  return (
    <div className="flex flex-col gap-4">
      <p className="text-[13px] text-ink-2">{t.intro}</p>

      <section className="flex flex-col gap-3">
        <div className="flex items-center gap-2">
          <h2 className="text-[14px] font-semibold">{t.keysTitle}</h2>
          <span className="flex-1" />
          <Button size="sm" variant="primary" onClick={() => setCreating(true)}><IconPlus size={13} /> {t.create}</Button>
        </div>
        {q.isLoading ? (
          <SkeletonRows rows={2} height={52} />
        ) : q.isError ? (
          <ErrorState error={q.error} onRetry={() => q.refetch()} />
        ) : (q.data ?? []).length === 0 ? (
          <EmptyState icon={<IconSettings size={28} />} title={t.empty} />
        ) : (
          <ul className="flex flex-col divide-y divide-line-row rounded-lg border border-line bg-surface">
            {(q.data ?? []).map((k) => (
              <li key={k.id} className="flex items-center gap-3 px-4 py-2.5">
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <span className="truncate text-[13.5px] font-semibold">{k.name}</span>
                  <span className="truncate text-[12px] text-ink-faint">
                    <code className="font-mono">{k.prefix}…</code> · {t.created(fmtDate(k.createdAt))} · {k.lastUsedAt ? t.lastUsed(fmtRelative(k.lastUsedAt)) : t.neverUsed}
                    {k.expiresAt ? ` · ${t.expiresOn(fmtDate(k.expiresAt))}` : ""}
                  </span>
                </div>
                <Button
                  size="sm"
                  variant="ghost"
                  loading={revoke.isPending && revoke.variables === k.id}
                  onClick={async () => {
                    if (await confirm({ title: t.revokeConfirm(k.name), message: t.revokeDesc, confirmLabel: t.revoke, danger: true })) revoke.mutate(k.id);
                  }}
                >
                  <IconTrash size={13} /> {t.revoke}
                </Button>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="flex flex-col gap-2 rounded-lg border border-line bg-surface p-4">
        <h2 className="text-[13px] font-semibold">{t.setupTitle}</h2>
        <p className="text-[12.5px] text-ink-2">{t.setupDesc}</p>
        <CopyBlock text={claudeCmd("<API_KEY>")} label={t.setupTitle} />
        <h2 className="mt-2 text-[13px] font-semibold">{t.setupJsonTitle}</h2>
        <p className="text-[12.5px] text-ink-2">{t.setupJsonDesc}</p>
        <CopyBlock text={jsonConfig("<API_KEY>")} label={t.setupJsonTitle} />
        <h2 className="mt-2 text-[13px] font-semibold">{t.toolsTitle}</h2>
        <p className="text-[12.5px] text-ink-2">{t.tools}</p>
        <h2 className="mt-2 text-[13px] font-semibold">{t.tryTitle}</h2>
        <p className="text-[12.5px] italic text-ink-2">{t.tryExample}</p>
      </section>

      {creating && (
        <CreateKeyModal
          onClose={() => setCreating(false)}
          onCreated={(k) => {
            setCreating(false);
            setCreated(k);
            qc.invalidateQueries({ queryKey: qk.apiKeys() });
          }}
        />
      )}
      {created && (
        <Modal
          open
          onClose={() => setCreated(null)}
          width={620}
          title={t.newKeyTitle}
          footer={<Button variant="primary" onClick={() => setCreated(null)}>{t.done}</Button>}
        >
          <div className="flex flex-col gap-3">
            <Alert tone="warning">{t.newKeyWarn}</Alert>
            <CopyBlock text={created.key} label={t.newKeyTitle} />
            <h3 className="text-[13px] font-semibold">{t.setupTitle}</h3>
            <CopyBlock text={claudeCmd(created.key)} label={t.setupTitle} />
          </div>
        </Modal>
      )}
    </div>
  );
}

function CreateKeyModal({ onClose, onCreated }: { onClose: () => void; onCreated: (k: CreatedAPIKeyResponse) => void }) {
  const tAll = useT();
  const t = tAll.apiKeys;
  const toast = useToast();
  const [name, setName] = useState("");
  const [days, setDays] = useState(0);
  const save = useMutation({
    mutationFn: () => postUsersMeApiKeys({ name: name.trim(), expiresInDays: days || undefined }),
    onSuccess: onCreated,
    onError: (e) => toast.error(e),
  });
  return (
    <Modal
      open
      onClose={onClose}
      width={480}
      title={t.create}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>{tAll.common.cancel}</Button>
          <Button variant="primary" disabled={!name.trim()} loading={save.isPending} onClick={() => save.mutate()}>{t.create}</Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <Field label={t.name} required>
          {(p) => <Input {...p} autoFocus maxLength={100} placeholder={t.namePlaceholder} value={name} onChange={(e) => setName(e.target.value)} />}
        </Field>
        <Field label={t.expires}>
          {(p) => (
            <Select {...p} value={days} onChange={(e) => setDays(Number(e.target.value))}>
              {EXPIRY_OPTIONS.map((d) => <option key={d} value={d}>{d === 0 ? t.never : t.days(d)}</option>)}
            </Select>
          )}
        </Field>
      </div>
    </Modal>
  );
}
