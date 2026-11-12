# Indstillinger

**Indstillinger** er fjorten fliser. Hver flise er en side med kort, og hver ændring skrives, mens du laver den, med et kort blink på kontrollen eller et ryst, når serveren afviser den. Nogle få vinduer har en **Gem**-knap, fordi det ville være usikkert at gemme dem halvfærdige, for eksempel en tilmelding eller loginadgangskoden. Forklaringer står i (i)-boblerne ved siden af hver kontrol.

Tryk på `/` eller Ctrl+K hvor som helst i indstillingerne for at søge i dem alle, også efter de navne, siderne havde i tidligere versioner. Rækkefølgen af fliserne kan ændres under **Udseende**. Hvor en kolonne af fliser ved siden af siden ville presse den, som i et vindue på omkring 1200 pixel eller på en telefon, bliver fliserne til en stribe over siden, der ruller sidelæns.

![Indstillinger med flisen Generelt åben.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Indstillinger med flisen Generelt åben.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Denne side viser hver flise og dens kort og linker til den side, der forklarer hvert enkelt i dybden.

## Generelt {#general}

- **Områder** slår hvert område til eller fra: **Containers**, **VMs**, **Flash**, **Mapper**, **ZFS-datasæt** og **Auto-sikkerhedskopi**. Et område, der er slået fra, sikkerhedskopieres ikke efter nogen tidsplan, heller ikke efter en posts egen plan, og dets poster tæller ikke. **Sikkerhedskopier nu** virker stadig. **Flash** vises kun på Unraid.
- **Containers** rummer det, der gælder for hver container, for eksempel **Automatiske databasedumps**. Alt, der påvirker én container, står på dens side. Se [Databasedumps](backups.md#database-dumps).

## Udseende {#look}

- **Udseende**: **Sprog** (42 sprog), **Tema** (**Mørkt**, **Lyst** eller **System**) med **OLED** nedenunder, som giver OLED- og AMOLED-skærme ægte sort, så længe det mørke tema vises, **Hjørner**, **Animation** (**Fra**, **Diskret**, **Vild** eller **Stormende**), **Stille notifikationer**, som skjuler succesmeldinger som bekræftelser af gem, og **Rækkefølgen af indstillingssiderne**, hvor du holder en flise og trækker den på plads eller bruger Alt og piletasterne.
- **Etiketter**: hvor meget af en kontrol der vises, hver for sig for **Knapper**, **Sidepanel**, **Faner** og **Bundpanel**: **Tekst**, **Tekst og symbol**, **Symbol** eller **Reaktiv**, som kun viser symbolet og glider ordene ind, når du peger på det. **Sidepanel** påvirker kun skrivebordslayoutet og **Bundpanel** kun telefonlayoutet.
- **Farver**: én **Accentfarve** eller **Regnbuetilstand**, hvor hver række i en liste får sin egen farve fra en palet på otte. Regnbuetilstand tilføjer **Diskotilstand**, hvor farverne glider langsomt til den næste, **Reaktiv tilstand**, hvor en rækkes farve kun vises, mens du peger på den, den kører eller er valgt, **Farverotation** og selve **Farvepaletten**. Tryllestaven forrest i accentfarverne, **Systemfarve**, tager din enheds egen farve: Material You i Android-appen fra Android 12, og systemfarven i Firefox og Safari. Den vises kun, hvor enheden har sådan en farve.

BombVault gemmer udseendet på serveren. En browser, der kommer for første gang, overtager det. **OLED** og **Systemfarve** er undtagelsen: hver enhed beholder dem for sig selv, da de afhænger af dens skærm. Når dit system beder om mindre bevægelse, vinder det altid over **Animation**.

## Tidsplan {#schedule}

**Uge**, **Fuld sikkerhedskopi** med **Hvornår sikkerhedskopierne kører**, **Indhent forpassede sikkerhedskopieringer efter start** og **Kommandoer før og efter**, **Tidsplaner pr. område** med **Off-site-kopier pr. område**, **Planlagte elementer**, **Rækkefølge** med **Genstart efter sikkerhedskopiering**, **Snapshot i stedet for stop**, **Opdateringer efter sikkerhedskopien** og **Ro før sikkerhedskopiering**. Se [Tidsplaner](schedules.md).

## Destinationer {#storage-locations}

- **Vis også i sidepanelet** fastgør siden Destinationer til sidepanelet.
- Listen over dine destinationer med **Tilføj destination**. Et klik åbner en destinations side, se [En destinations side](offsite-recovery.md#location-page).
- **Adgangsoplysninger**: de gemte adgangsoplysninger med standardstjernen, se [Adgangsoplysninger](offsite-recovery.md#sign-ins).
- **Destinationer pr. område**: 3-2-1-flowet, hvor hver linje er en kontakt, se [Standarder pr. område](offsite-recovery.md#placement-defaults).
- **Depoter**: repositories lavet til enkelte poster, se [Placering pr. element](offsite-recovery.md#placement).
- **Hvor længe der gemmes** peger på destinationerne, hvor opbevaring indstilles, og **Forhåndsvisning** viser **Hvad den næste kørsel ville fjerne**, pr. repository og pr. post, før det sker. Den ændrer intet og låser intet, så den svarer også, mens en sikkerhedskopiering kører.
- **Eksterne kopier**: én række pr. område med **Test forbindelse**, **Replikér nu** og **Opsæt** eller **Rediger**, se [Off-site-kopier](offsite-recovery.md#off-site-copies).
- **Off-site-båndbredde**: **Upload-grænse (KiB/s)** og **Download-grænse (KiB/s)** for hver kopi. 0 betyder ubegrænset.

Kortet **Streaming**, som sænker upload-grænsen, mens Plex, Jellyfin eller Emby streamer, står på selve siden Destinationer.

## Gendannelse {#restore}

- Et link til siden **Gendannelse**.
- **Mapper til gendannelser**: **Standardmappe til gendannelse** for en gendannelse til en mappe, med **Undermappe pr. område** og en **Undtagelse** pr. område. Se [Hvor en gendannelse til en mappe havner](restore.md#folders).
- **Container-registre**: adgangsoplysninger til private registre, for eksempel et `ghcr.io`-image bag en sponsorbarriere. En gendannelse bruger dem til at hente et manglende image, og en opdatering efter sikkerhedskopien til at se efter et nyere. Offentlige images hentes uden. **Tilføj register** tilbyder offentlige registre, selvhostede, dem hos en cloududbyder og **Andet register** til enhver adresse.
- **Til nødsituationer**: **Download gendannelseskit**, som kræver en loginadgangskode. Se [Gendannelseskit](offsite-recovery.md#recovery-kit).

## Notifikationer {#notifications}

**Notifikationer** med opsummeringerne, **Meld forsinkede sikkerhedskopier** og den **Ugentlige oversigt**, og **Kanaler**, en liste over Unraid-, ntfy-, Gotify-, Discord-, Slack-, Matrix-, e-mail-, Apprise-, webhook- og Healthchecks-kanaler, hver med sin egen kontakt, **Underret** og **Send test**. Se [Notifikationer](notifications.md).

## Verificering {#verify}

- **Tjek sikkerhedskopier**: resultatet af det sidste `restic check` pr. område, med **Lås op** og **Beskær** under **Mere** i hver række. **Lås op** rydder en forældet lås og lader en lås, der stadig bliver fornyet, være, og **Beskær** anvender behold-reglerne på forlangende.
- **Automatiske tjek**: det planlagte gendannelsestjek med sin **Datastikprøve (%)** og testmål, **Starttest**, som også gendanner én container til en isoleret kopi og starter den, den **Planlagte off-site DR-test** og **Tjek sletningsbeskyttelse**, den planlagte manipulationstest. Se [DR-test](offsite-recovery.md#dr-drills) og [Manipulationstest](offsite-recovery.md#tamper-test).
- **Afvigelser**: registrering, følsomhed og hvilke fund der sender en besked. Se [Afvigelser](anomalies.md).

## Sikkerhed {#security}

- **Adgangskode**: loginadgangskoden, mindst 12 tegn. At ændre eller rydde den spørger efter den nuværende og, med tofaktorgodkendelse slået til, koden.
- **Tofaktorgodkendelse**: en kode fra en autentificeringsapp oven i adgangskoden, med otte gendannelseskoder, der vises én gang.
- **Adgangsnøgler**: log ind med en nøgle på din telefon, laptop eller sikkerhedspind. De kræver et rigtigt domænenavn og et certifikat, browseren stoler på.
- **Android-app**: **Telefoner kan parre med gruppens tolv ord**, hvilket lader en parret telefon logge ind uden adgangskoden.
- **Kryptering**: **Kryptér sikkerhedskopier**, slået til som standard.

Flere ting kræver en loginadgangskode, for eksempel gendannelseskittet og nøgledepoter. Se [Sikkerhedsmodel](configuration.md#security-model).

## Instanser {#instances}

- **Vis også i sidepanelet** fastgør siden Instanser til sidepanelet.
- **Flere servere** slår delene af en gruppe til: **Instanser**, siden med et kort for hvert medlem, **Modtager** og **Hentning**. Se [Instanser og roller](offsite-recovery.md#roles).
- Et link til siden Instanser med de dele, der er slået til.

## Parring {#pairing}

**Parring** med **Generer sætning** eller **Indtast sætning**, trinkortene, denne instans' navn, de enheder, der venter på **Godkend**, kortet **Videresender** med sine fire former og **Hvad videresenderen ser**, gruppens medlemmer og **ZFS-server uden BombVault** til et replikamål over SSH. Under dem forbinder **Fæstning** BombVault med de andre apps i familien via fæstningens tolv ord, med en kontakt for hver tilladelse mellem apps, **Giv sikkerhedskopier tilbage til apps**, **Vis sætning**, **Skift** og **Forlad**. Se [Parring af instanser](offsite-recovery.md#pairing), [Fæstningen](offsite-recovery.md#fortress) og [Send til en ZFS-server](zfs-datasets.md#replica-push).

## Integrationer {#integrations}

- **MCP-server**: nøgler til AI-assistenter, **Vælg en klient** og hver nøgles log. Se [MCP-server](mcp.md).
- **API-tokens**: navngivne tokens til scripts og dashboards. Se [Tokens](api.md#tokens).
- **Tjenester**: **Home Assistant** over MQTT, **Find på netværket** (mDNS som `bombvault.local`) og **Eksponer /metrics** til Prometheus med et **Scrape-token (valgfrit)**. Se [Home Assistant](api.md#home-assistant) og [Find BombVault på netværket](api.md#mdns).
- **Dashboard-widget**: et skrivebeskyttet token og en adresse til ethvert dashboard, der viser en iframe, for eksempel Homepage eller Heimdall, med kun den levende aktivitetslog. Se [Dashboard-widget](api.md#dashboard-widget).
- **Værts-SSH**: BombVaults offentlige nøgle og **Test forbindelse**, til VM- og ZFS-datasætsikkerhedskopier og Unraid-notifikationer. Se [VM-guiden](vm-backup-ssh-setup.md).

## Apps {#apps}

- **Android-app**: APK'en til den udgave, din server kører, med en QR-kode, og F-Droid-opslaget. Se [Android-app](android.md#install).
- **ParleyPort**: relay-containeren med dens Unraid-skabelon og Docker-kommandoen, der starter den.
- **TollRoll**: én flise på Unraid Dashboard til BombVault og de andre apps i din fæstning. Installér den fra Community Applications, og lad den slutte sig til fæstningen. Den læser apps gennem fæstningen og har ikke brug for et token. Se [Fæstningen](offsite-recovery.md#fortress).

## System {#system}

- **Eksportér / importér indstillinger**: hele din konfiguration som en bærbar JSON-fil, med eller uden adgangsoplysningerne. Import viser først **Denne fil indeholder** og udelader links til destinationer, der er væk. **Krypter eksporter med age** forsegler de almindelige eksporter, flash-ZIP'en og gendannelseskittet til de **age-modtagere**, du angiver. Se [Bærbare indstillinger](configuration.md#portable-settings-export-and-import).
- **Ydeevne**: **CPU-tråde (0 = alle)**, **Grænse for cachestørrelse (MB, 0 = ubegrænset)** til restics cache under `/config` og **Hukommelsesgrænse pr. sikkerhedskopi (MiB, 0 = ingen)**, en blød grænse, der får en stor sikkerhedskopiering til at gå langsommere i stedet for at blive dræbt for at løbe tør for hukommelse.

## Info {#info}

- **Version**: BombVault, GlimStone, restic og rclone, med **Søg efter opdateringer** og **Ny**, som tæller de ændringer, du endnu ikke har set, og åbner et vindue, der viser dem.
- **Hjælp**: **Manual** åbner denne dokumentation, og **Fejlrapport** downloader en redigeret ZIP til et GitHub-issue. Den kræver en loginadgangskode.
- **Om BombVault**: projektet, dets licens og hvordan du når forfatteren.
