import { useEffect, useMemo, useState } from "react";
import { takeSignInCode, type SignInTileCard, type ToastSeverity } from "@glimstone/core";
import { IDENTITY_PROVIDERS } from "@glimstone/core/identityProviders";
import { SignInTile } from "@glimstone/react/SignInTile";
import { explainedError, getPublicAddress, setPublicAddress } from "../../../lib/api";
import { useT } from "../../../lib/i18n";
import { formatTs } from "../../../lib/reltime";
import { signIn } from "../../../lib/signInKit";
import { tileText } from "../../../lib/signInWords";
import { useToast } from "../../../lib/toast";
import { Rows, SettingRow } from "../../../components/storage/rows";
import { EncryptionCard } from "../EncryptionCard";
import { Card, hueCounter } from "../shared";

// API tokens are BombVault's own and live under Integrations beside the MCP keys, so the tile leaves the kit's out.
const CARDS: SignInTileCard[] = ["password", "askPassword", "twoFactor", "passkeys", "sessions", "services", "accounts"];

export function SecurityPage() {
  const { t, lang } = useT();
  const { push } = useToast();
  const text = useMemo(() => tileText(t, lang), [t, lang]);
  const [arrived] = useState(takeSignInCode);
  const notify = (message: string, severity: ToastSeverity) => push(message, severity === "fail" ? "fail" : severity === "warn" ? "warn" : "success");
  const nextHue = hueCounter();
  const addressHue = nextHue();
  const tileHue = nextHue();

  return (
    <>
      <AddressCard hueIndex={addressHue} />
      <SignInTile
        api={signIn}
        text={text}
        lang={lang}
        providers={IDENTITY_PROVIDERS}
        cards={CARDS}
        notify={notify}
        arrived={arrived}
        time={(iso) => formatTs(Math.floor(Date.parse(iso) / 1000))}
        hueStart={tileHue}
      />
      {/* The tile takes at most one hue for each of its cards. */}
      <EncryptionCard hueIndex={tileHue + CARDS.length} />
    </>
  );
}

/** AddressCard keeps where people reach this instance, which passkeys and sign-in services need. It saves as soon
 *  as the field is left. */
function AddressCard({ hueIndex }: { hueIndex: number }) {
  const { t } = useT();
  const { push } = useToast();
  const [saved, setSaved] = useState<string | null>(null);
  const [value, setValue] = useState("");

  useEffect(() => {
    let live = true;
    void getPublicAddress()
      .then((r) => {
        if (live && r.ok) {
          setSaved(r.address ?? "");
          setValue(r.address ?? "");
        }
      })
      .catch(() => undefined);
    return () => {
      live = false;
    };
  }, []);

  async function save() {
    if (saved === null || value.trim() === saved) return;
    try {
      const r = await setPublicAddress(value.trim());
      if (!r.ok) {
        push(r.code === "address-invalid" ? t("signin.address.invalid") : (r.error ?? t("auth.saveError")), "fail");
        return;
      }
      setSaved(r.address ?? "");
      setValue(r.address ?? "");
      push(r.address ? t("signin.address.saved") : t("signin.address.cleared"), "success");
    } catch (err) {
      push(explainedError(err) ?? t("auth.saveError"), "fail");
    }
  }

  if (saved === null) return null;
  return (
    <Card title={t("signin.address.title")} hint={t("signin.address.hint")} hueIndex={hueIndex}>
      <Rows>
        <SettingRow label={t("signin.address.label")} htmlFor="bv-public-address">
          <input
            id="bv-public-address"
            type="url"
            inputMode="url"
            dir="ltr"
            spellCheck={false}
            autoCapitalize="off"
            placeholder="https://backup.example"
            value={value}
            onChange={(e) => setValue(e.target.value)}
            onBlur={() => void save()}
            onKeyDown={(e) => {
              if (e.key === "Enter") void save();
            }}
            className="w-80 max-w-full rounded-control bg-carbon-surface2 px-3 py-1.5 text-start text-sm text-carbon-text glim-field-focus"
          />
        </SettingRow>
      </Rows>
    </Card>
  );
}
