"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { useMutation, useQuery } from "@tanstack/react-query";
import { getOauthClient, postOauthAuthorize } from "@/lib/api/endpoints";
import { useAuth } from "@/lib/auth/AuthContext";
import { useT } from "@/lib/i18n";
import { Button } from "@/components/ui/Button";
import { Alert, SkeletonRows } from "@/components/ui/feedback";

/** OAuth consent screen for MCP connectors (Claude Desktop, claude.ai). The proxy sends signed-out
 *  visitors to /login first and brings them back here with the same query. */
export function OAuthConsent() {
  const t = useT().oauthConsent;
  const sp = useSearchParams();
  const { user, logout } = useAuth();
  const [leaving, setLeaving] = useState(false);

  const clientId = sp.get("client_id") ?? "";
  const redirectUri = sp.get("redirect_uri") ?? "";
  const state = sp.get("state") ?? "";
  const wellFormed = sp.get("response_type") === "code" && !!clientId && !!redirectUri && !!sp.get("code_challenge");

  // The server checks that redirect_uri belongs to the client before we ever send the user there.
  const client = useQuery({
    queryKey: ["me", "oauth-client", clientId, redirectUri],
    queryFn: () => getOauthClient(clientId, { redirectUri }),
    enabled: wellFormed,
    retry: false,
  });

  const allow = useMutation({
    mutationFn: () =>
      postOauthAuthorize({
        clientId,
        redirectUri,
        codeChallenge: sp.get("code_challenge") ?? "",
        codeChallengeMethod: sp.get("code_challenge_method") ?? "",
        state: state || undefined,
        scope: sp.get("scope") || undefined,
      }),
    onSuccess: (r) => {
      setLeaving(true);
      window.location.assign(r.redirectUrl);
    },
  });

  const deny = () => {
    const u = new URL(redirectUri);
    u.searchParams.set("error", "access_denied");
    if (state) u.searchParams.set("state", state);
    setLeaving(true);
    window.location.assign(u.toString());
  };

  const invalid = !wellFormed || client.isError;

  return (
    <main className="flex min-h-dvh items-center justify-center bg-canvas px-4 py-10">
      <div className="flex w-full max-w-[440px] flex-col gap-5 rounded-xl border border-line bg-surface p-6 shadow-sm">
        <div className="flex items-center gap-2">
          <div style={{ background: "var(--brand-grad)" }} className="flex size-8 items-center justify-center rounded-lg text-sm font-bold text-white">K</div>
          <span className="text-lg font-bold tracking-tight text-ink">KAIRSO</span>
        </div>

        {invalid ? (
          <div className="flex flex-col gap-2">
            <h1 className="text-[18px] font-semibold text-ink">{t.invalid}</h1>
            <p className="text-[13px] text-ink-2">{t.invalidDesc}</p>
          </div>
        ) : client.isLoading || !client.data ? (
          <SkeletonRows rows={3} height={20} />
        ) : (
          <>
            <div className="flex flex-col gap-2">
              <h1 className="text-[18px] font-semibold text-ink">{t.title(client.data.name)}</h1>
              <p className="text-[13px] text-ink-2">{t.body(user?.email || user?.name || "")}</p>
              <p className="text-[12.5px] text-ink-muted">{t.redirectNote(client.data.redirectHost)}</p>
              <p className="text-[12.5px] text-ink-muted">{t.revokeNote}</p>
            </div>
            {allow.isError && <Alert tone="danger">{allow.error instanceof Error ? allow.error.message : String(allow.error)}</Alert>}
            <div className="flex justify-end gap-2">
              <Button variant="ghost" disabled={leaving || allow.isPending} onClick={deny}>{t.deny}</Button>
              <Button variant="primary" loading={allow.isPending || leaving} onClick={() => allow.mutate()}>{t.allow}</Button>
            </div>
            <button type="button" onClick={() => logout()} className="self-start text-[12px] text-ink-muted underline-offset-2 hover:underline">
              {t.notMe}
            </button>
          </>
        )}
      </div>
    </main>
  );
}
