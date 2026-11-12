# Innstillinger

**Innstillinger** består av fjorten fliser. Hver flis er en side med kort, og hver endring skrives mens du gjør den, med et kort glimt på kontrollen, eller et rist når serveren avviser den. Noen få vinduer beholder en **Lagre**-knapp fordi det ville vært utrygt å lagre dem halvferdige, for eksempel en pålogging eller innloggingspassordet. Forklaringer står i (i)-boblene ved siden av hver kontroll.

Trykk `/` eller Ctrl+K hvor som helst i innstillingene for å søke i alle, også etter navnene sider hadde i tidligere versjoner. Rekkefølgen på flisene kan endres under **Utseende**. Der en flisrekke ved siden av siden ville presse den sammen, som i et vindu rundt 1200 piksler bredt eller på en telefon, blir flisene en stripe over siden som ruller sideveis.

![Innstillinger med flisen Generelt åpen.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Innstillinger med flisen Generelt åpen.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Denne siden lister hver flis og kortene i den, og lenker til siden som forklarer hver av dem i dybden.

## Generelt {#general}

- **Områder** slår hvert område av eller på: **Kontainere**, **VM-er**, **Flash**, **Mapper**, **ZFS-datasett** og **Auto-sikkerhetskopi**. Et område som er av, sikkerhetskopieres ikke etter noen tidsplan, heller ikke etter den egne planen til et element, og elementene i det telles ikke. **Sikkerhetskopier nå** virker fortsatt. **Flash** vises bare på Unraid.
- **Kontainere** har det som gjelder for alle containere, for eksempel **Automatiske databasedumper**. Alt som påvirker én container, står på siden til containeren. Se [Databasedumper](backups.md#database-dumps).

## Utseende {#look}

- **Utseende**: **Språk** (42 språk), **Tema** (**Mørkt**, **Lyst** eller **System**) med **OLED** under, som gir OLED- og AMOLED-skjermer ekte svart så lenge det mørke temaet vises, **Hjørner**, **Animasjon** (**Av**, **Diskret**, **Vill** eller **Stormende**), **Stille varsler**, som skjuler suksessmeldinger som lagringsbekreftelser, og **Rekkefølgen på innstillingssidene**, der du holder en flis og drar den på plass, eller bruker Alt og piltastene.
- **Etiketter**: hvor mye av en kontroll som vises, separat for **Knapper**, **Sidefelt**, **Faner** og **Bunnfelt**: **Tekst**, **Tekst og symbol**, **Symbol** eller **Reaktiv**, som viser bare symbolet og skyver ordene inn når du peker på det. **Sidefelt** påvirker bare skrivebordsvisningen og **Bunnfelt** bare telefonvisningen.
- **Farger**: én **Aksentfarge**, eller **Regnbuemodus**, der hver rad i en liste får sin egen farge fra en palett på åtte. Regnbuemodus legger til **Diskomodus**, der fargene glir sakte over i den neste, **Reaktiv modus**, der en rads farge bare vises mens du peker på den, den kjører eller er valgt, **Fargerotasjon** og selve **Fargepalett**. Tryllestaven først blant aksentfargene, **Systemfarge**, tar enhetens egen farge: Material You i Android-appen fra Android 12, og systemfargen i Firefox og Safari. Den vises bare der enheten har en slik farge.

BombVault husker utseendet på serveren. En nettleser som kommer for første gang, overtar det. **OLED** og **Systemfarge** er unntaket: hver enhet beholder dem for seg selv, siden de avhenger av skjermen. Når systemet ditt ber om mindre bevegelse, vinner det alltid over **Animasjon**.

## Tidsplan {#schedule}

**Uke**, **Full sikkerhetskopi** med **Når sikkerhetskopiene kjører**, **Ta igjen tapte sikkerhetskopier etter oppstart** og **Kommandoer før og etter**, **Tidsplaner per område** med **Off-site-kopier per område**, **Planlagte elementer**, **Rekkefølge** med **Omstart etter sikkerhetskopiering**, **Øyeblikksbilde i stedet for stopp**, **Oppdateringer etter sikkerhetskopien** og **Ro før sikkerhetskopi**. Se [Tidsplaner](schedules.md).

## Mål {#storage-locations}

- **Vis også i sidefeltet** fester siden Mål til sidefeltet.
- Listen over målene dine, med **Legg til mål**. Et klikk åpner siden til et mål, se [Siden til et mål](offsite-recovery.md#location-page).
- **Pålogginger**: de lagrede påloggingene med standardstjernen, se [Pålogginger](offsite-recovery.md#sign-ins).
- **Mål per område**: 3-2-1-flyten, der hver linje er en bryter, se [Standarder per område](offsite-recovery.md#placement-defaults).
- **Depoter**: depoter laget for enkeltelementer, se [Plassering per element](offsite-recovery.md#placement).
- **Hvor lenge det beholdes** peker til målene, der oppbevaringen settes, og **Forhåndsvisning** viser **Hva neste kjøring ville fjerne**, per depot og per element, før det skjer. Den endrer ingenting og låser ingenting, så den svarer også mens en sikkerhetskopi kjører.
- **Eksterne kopier**: én rad per område med **Test tilkobling**, **Replikér nå** og **Sett opp** eller **Rediger**, se [Eksterne kopier](offsite-recovery.md#off-site-copies).
- **Ekstern båndbredde**: **Opplastingsgrense (KiB/s)** og **Nedlastingsgrense (KiB/s)** for alle kopier. 0 betyr ubegrenset.

Kortet **Strømming**, som senker opplastingsgrensen mens Plex, Jellyfin eller Emby strømmer, ligger på selve siden Mål.

## Gjenoppretting {#restore}

- En lenke til siden **Gjenoppretting**.
- **Mapper for gjenoppretting**: **Standard gjenopprettingsmappe** for en gjenoppretting til en mappe, med **Undermappe per område** og et **Unntak** per område. Se [Hvor en gjenoppretting til en mappe havner](restore.md#folders).
- **Container-registre**: pålogginger for private registre, for eksempel et `ghcr.io`-image bak sponsorlås. En gjenoppretting bruker dem til å hente et manglende image, og en oppdatering etter sikkerhetskopien til å se etter et nyere. Offentlige images hentes uten. **Legg til register** tilbyr offentlige registre, selvhostede, registre hos en skyleverandør og **Annet register** for hvilken som helst adresse.
- **For nødstilfeller**: **Last ned gjenopprettingssett**, som krever et innloggingspassord. Se [Gjenopprettingssett](offsite-recovery.md#recovery-kit).

## Varsler {#notifications}

**Varsler** med oppsummeringene, **Varsle om forsinkede sikkerhetskopier** og **Ukentlig oppsummering**, og **Kanaler**, en liste over Unraid-, ntfy-, Gotify-, Discord-, Slack-, Matrix-, e-post-, Apprise-, webhook- og Healthchecks-kanaler, hver med sin egen bryter, **Varsle** og **Send test**. Se [Varsler](notifications.md).

## Verifisering {#verify}

- **Kontroller sikkerhetskopier**: resultatet av den siste `restic check` per område, med **Lås opp** og **Beskjær** under **Mer** i hver rad. **Lås opp** fjerner en gammel lås og lar en som fortsatt blir fornyet, være i fred, og **Beskjær** bruker beholdningsreglene på forespørsel.
- **Automatiske kontroller**: den planlagte gjenopprettingskontrollen med **Datautvalg (%)** og testmål, **Starttest**, som også gjenoppretter én container til en isolert kopi og starter den, den **Planlagt off-site DR-øvelse** og **Kontroller slettebeskyttelse**, den planlagte tamper-testen. Se [DR-øvelser](offsite-recovery.md#dr-drills) og [Tamper-test](offsite-recovery.md#tamper-test).
- **Avvik**: oppdagelse, følsomhet og hvilke funn som sender en melding. Se [Avvik](anomalies.md).

## Sikkerhet {#security}

- **Passord**: innloggingspassordet, minst 12 tegn. Å endre eller fjerne det krever det nåværende og, med tofaktorautentisering på, koden.
- **Tofaktorautentisering**: en kode fra en autentiseringsapp i tillegg til passordet, med åtte gjenopprettingskoder som vises én gang.
- **Passnøkler**: logg inn med en nøkkel på telefonen, laptopen eller en sikkerhetsnøkkel. De trenger et ekte domenenavn og et sertifikat nettleseren stoler på.
- **Android-app**: **Telefoner kan pare med gruppens tolv ord**, som lar en paret telefon logge inn uten passordet.
- **Kryptering**: **Krypter sikkerhetskopier**, på som standard.

Flere ting krever et innloggingspassord, som gjenopprettingssettet og nøkkeldeponering. Se [Sikkerhetsmodell](configuration.md#security-model).

## Instanser {#instances}

- **Vis også i sidefeltet** fester siden Instanser til sidefeltet.
- **Flere servere** slår på delene av en gruppe: **Instanser**, siden med et kort for hvert medlem, **Mottaker** og **Henting**. Se [Instanser og roller](offsite-recovery.md#roles).
- En lenke til siden Instanser, med delene som er på.

## Paring {#pairing}

**Paring** med **Generer frase** eller **Angi frase**, trinnkortene, navnet på denne instansen, enhetene som venter på **Godkjenn**, kortet **Relay** med sine fire former og **Hva relayet ser**, medlemmene i gruppen, og **ZFS-server uten BombVault** for et replikamål over SSH. Under dem kobler **Festning** BombVault til de andre appene i familien med festningens tolv ord, med en bryter for hver tillatelse mellom appene, **Gi sikkerhetskopier tilbake til apper**, **Vis frase**, **Bytt** og **Forlat**. Se [Paring av instanser](offsite-recovery.md#pairing), [Festningen](offsite-recovery.md#fortress) og [Send til en ZFS-server](zfs-datasets.md#replica-push).

## Integrasjoner {#integrations}

- **MCP-server**: nøkler for AI-assistenter, **Velg en klient** og loggen til hver nøkkel. Se [MCP-server](mcp.md).
- **API-tokener**: navngitte tokener for skript og dashbord. Se [Tokener](api.md#tokens).
- **Tjenester**: **Home Assistant** over MQTT, **Finn på nettverket** (mDNS som `bombvault.local`) og **Eksponer /metrics** for Prometheus, med et **Scrape-token (valgfritt)**. Se [Home Assistant](api.md#home-assistant) og [Finn BombVault på nettverket](api.md#mdns).
- **Dashbord-widget**: et skrivebeskyttet token og en adresse for et hvilket som helst dashbord som viser en iframe, for eksempel Homepage eller Heimdall, med bare den levende aktivitetsloggen. Se [Dashbord-widget](api.md#dashboard-widget).
- **Verts-SSH**: BombVaults offentlige nøkkel og **Test tilkobling**, for sikkerhetskopier av VM-er og ZFS-datasett og for Unraid-varsler. Se [VM-veiledningen](vm-backup-ssh-setup.md).

## Apper {#apps}

- **Android-app**: APK-en for utgaven serveren din kjører, med en QR-kode, og F-Droid-oppføringen. Se [Android-app](android.md#install).
- **ParleyPort**: relay-containeren med Unraid-malen og Docker-kommandoen som starter den.
- **TollRoll**: én flis på Unraid-dashbordet for BombVault og de andre appene i festningen din. Installer den fra Community Applications og la den bli med i festningen. Den leser appene gjennom festningen og trenger ikke noe token. Se [Festningen](offsite-recovery.md#fortress).

## System {#system}

- **Eksporter / importer innstillinger**: hele konfigurasjonen din som en flyttbar JSON-fil, med eller uten påloggingsdataene. Import viser **Denne filen inneholder** først og utelater lenker til mål som er borte. **Krypter eksporter med age** forsegler de rene eksportene, flash-ZIP-en og gjenopprettingssettet for **age-mottakere** du lister opp. Se [Flyttbare innstillinger](configuration.md#portable-settings-export-and-import).
- **Ytelse**: **CPU-tråder (0 = alle)**, **Grense for bufferstørrelse (MB, 0 = ubegrenset)** for restics buffer under `/config`, og **Minnegrense per sikkerhetskopi (MiB, 0 = ingen)**, en myk grense som får en stor sikkerhetskopi til å gå saktere i stedet for å bli drept for å gå tom for minne.

## Info {#info}

- **Versjon**: BombVault, GlimStone, restic og rclone, med **Se etter oppdateringer** og **Nytt**, som teller endringene du ikke har sett ennå og åpner et vindu som lister dem.
- **Hjelp**: **Håndbok** åpner denne dokumentasjonen, og **Feilrapport** laster ned en renset ZIP for en GitHub-sak. Den krever et innloggingspassord.
- **Om BombVault**: prosjektet, lisensen og hvordan du når utvikleren.
