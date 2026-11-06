# Inställningar

**Inställningar** består av fjorton brickor. Varje bricka är en sida med kort, och varje ändring skrivs medan du gör den, med en kort blinkning på reglaget, eller ett skakande när servern nekar. Några fönster behåller en knapp **Spara** eftersom det vore osäkert att spara dem halvfärdiga, till exempel en inloggning eller inloggningslösenordet. Förklaringar finns i (i)-bubblorna bredvid varje reglage.

Tryck `/` eller Ctrl+K var som helst i inställningarna för att söka i alla, även efter de namn sidorna hade i tidigare versioner. Brickornas ordning kan ändras under **Utseende**. Där en kolumn med brickor bredvid sidan skulle klämma ihop den, som i ett fönster på omkring 1200 pixlar eller på en telefon, blir brickorna en list ovanför sidan som rullar i sidled.

![Inställningar med brickan Allmänt öppen.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Inställningar med brickan Allmänt öppen.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Den här sidan listar varje bricka och dess kort, och länkar till sidan som förklarar var och en på djupet.

## Allmänt {#general}

- **Avsnitt** slår på eller av varje avsnitt: **Containers**, **VMs**, **Flash**, **Mappar**, **ZFS-datauppsättningar** och **Auto-säkerhetskopia**. Ett avsnitt som är av säkerhetskopieras inte enligt något schema, inte heller enligt en posts egen plan, och dess poster räknas inte. **Säkerhetskopiera nu** fungerar fortfarande. **Flash** visas bara på Unraid.
- **Containers** innehåller det som gäller för alla containrar, till exempel **Automatiska databasdumpar**. Allt som rör en enda container finns på dess sida. Se [Databasdumpar](backups.md#database-dumps).

## Utseende {#look}

- **Utseende**: **Språk** (42 språk), **Tema** (**Mörkt**, **Ljust** eller **System**), **Hörn**, **Animation** (**Av**, **Subtil**, **Vild** eller **Stormig**), **Tysta aviseringar**, som döljer lyckade meddelanden som sparbekräftelser, och **Ordning på inställningssidorna**, där du håller en bricka och drar den på plats, eller använder Alt och piltangenterna.
- **Etiketter**: hur mycket av ett reglage som visas, separat för **Knappar**, **Sidofält**, **Flikar** och **Nedre fält**: **Text**, **Text och symbol**, **Symbol** eller **Reaktiv**, som visar bara symbolen och låter orden glida in när du pekar på den. **Sidofält** påverkar bara skrivbordslayouten och **Nedre fält** bara telefonlayouten.
- **Färger**: en **Accentfärg**, eller **Regnbågsläge**, där varje rad i en lista får sin egen färg från en palett på åtta. Regnbågsläget lägger till **Discoläge**, där färgerna sakta glider till nästa, **Reaktivt läge**, där en rads färg visas bara medan du pekar på den, den körs eller är vald, **Färgrotation** och själva **Färgpaletten**.

BombVault sparar utseendet på servern. En webbläsare som kommer för första gången tar över det. När ditt system ber om mindre rörelse gäller det alltid före **Animation**.

## Schema {#schedule}

**Vecka**, **Total säkerhetskopia** med **När säkerhetskopiorna körs**, **Kör ikapp missade säkerhetskopieringar efter start** och **Kommandon före och efter**, **Scheman per avsnitt** med **Externa kopior per avsnitt**, **Schemalagda poster**, **Ordning** med **Starta om efter säkerhetskopiering**, **Ögonblicksbild i stället för stopp**, **Uppdateringar efter säkerhetskopian** och **Vila före säkerhetskopia**. Se [Scheman](schedules.md).

## Lagringsmål {#storage-locations}

- **Visa även i sidofältet** fäster sidan Lagringsmål i sidofältet.
- Listan över dina lagringsmål, med **Lägg till mål**. Ett klick öppnar ett måls sida, se [Ett lagringsmåls sida](offsite-recovery.md#location-page).
- **Inloggningar**: de sparade inloggningarna med standardstjärnan, se [Inloggningar](offsite-recovery.md#sign-ins).
- **Mål per avsnitt**: 3-2-1-flödet, där varje rad är en omkopplare, se [Standard per avsnitt](offsite-recovery.md#placement-defaults).
- **Arkiv**: repositorier som gjorts för enskilda poster, se [Placering per post](offsite-recovery.md#placement).
- **Hur länge det bevaras** pekar på lagringsmålen, där gallringen ställs in, och **Förhandsgranska** visar **Vad nästa körning skulle ta bort**, per repositorium och per post, innan det händer. Det ändrar ingenting och låser ingenting, så det svarar även medan en säkerhetskopiering pågår.
- **Externa kopior**: en rad per avsnitt med **Testa anslutning**, **Replikera nu** och **Konfigurera** eller **Redigera**, se [Externa kopior](offsite-recovery.md#off-site-copies).
- **Off-site-bandbredd**: **Uppladdningsgräns (KiB/s)** och **Nedladdningsgräns (KiB/s)** för varje kopia. 0 betyder obegränsat.

Kortet **Streaming**, som sänker uppladdningsgränsen medan Plex, Jellyfin eller Emby strömmar, finns på sidan Lagringsmål.

## Återställning {#restore}

- En länk till sidan **Återställning**.
- **Mappar för återställning**: **Standardmapp för återställning** för en återställning till en mapp, med **Undermapp per avsnitt** och ett **Undantag** per avsnitt. Se [Vart en återställning till en mapp går](restore.md#folders).
- **Container-register**: inloggningar för privata register, till exempel en `ghcr.io`-image bakom sponsring. En återställning använder dem för att hämta en saknad image, och en uppdatering efter säkerhetskopian för att leta efter en nyare. Publika images hämtas utan. **Lägg till register** erbjuder publika register, egna register, register hos en molnleverantör och **Annat register** för vilken adress som helst.
- **För nödfall**: **Ladda ner återställningskit**, som kräver ett inloggningslösenord. Se [Återställningskit](offsite-recovery.md#recovery-kit).

## Aviseringar {#notifications}

**Aviseringar** med **Avisera**, sammanfattningarna, **Rapportera försenade säkerhetskopior**, **Unraid-aviseringar** och **Veckosammanfattning**, **Kanaler** med Webhook, Apprise, Matrix, Gotify och E-post (SMTP), och **Healthchecks**. Se [Aviseringar](notifications.md).

## Verifiera {#verify}

- **Kontrollera säkerhetskopior**: resultatet av den senaste `restic check` per avsnitt, med **Lås upp** och **Rensa** under **Mer** på varje rad. **Lås upp** rensar ett gammalt lås, och **Rensa** tillämpar behållreglerna på begäran.
- **Automatiska kontroller**: den schemalagda återställningskontrollen med sitt **Dataurval (%)** och sina testmål, **Starttest**, som också återställer en container till en isolerad kopia och startar den, den **Schemalagda off-site DR-övningen** och **Kontrollera raderingsskydd**, det schemalagda manipuleringstestet. Se [DR-övningar](offsite-recovery.md#dr-drills) och [Manipuleringstest](offsite-recovery.md#tamper-test).
- **Avvikelser**: upptäckt, känslighet och vilka fynd som skickar ett meddelande. Se [Avvikelser](anomalies.md).

## Säkerhet {#security}

- **Lösenord**: inloggningslösenordet, minst 12 tecken. Att ändra eller ta bort det kräver det nuvarande och, med tvåfaktorsautentisering på, koden.
- **Tvåfaktorsautentisering**: en kod från en autentiseringsapp utöver lösenordet, med åtta återställningskoder som visas en gång.
- **Lösennycklar**: logga in med en nyckel på telefonen, laptopen eller en säkerhetsnyckel. De kräver ett riktigt domännamn och ett certifikat som webbläsaren litar på.
- **Android-app**: **Telefoner kan parkopplas med gruppens tolv ord**, vilket låter en parkopplad telefon logga in utan lösenordet.
- **Kryptering**: **Kryptera säkerhetskopior**, på som standard.

Flera saker kräver ett inloggningslösenord, till exempel återställningskitet och nyckeldepositioner. Se [Säkerhetsmodell](configuration.md#security-model).

## Instanser {#instances}

- **Visa även i sidofältet** fäster sidan Instanser i sidofältet.
- **Flera servrar** slår på delarna av en grupp: **Instanser**, sidan med ett kort för varje medlem, **Mottagare** och **Hämtning**. Se [Instanser och roller](offsite-recovery.md#roles).
- En länk till sidan Instanser, med de delar som är på.

## Parkoppling {#pairing}

**Parkoppling** med **Skapa fras** eller **Ange fras**, stegkorten, den här instansens namn och **Kända adresser**, kortet **Relä** med sina fyra former och **Vad reläet ser**, gruppens medlemmar, och **ZFS-server utan BombVault** för ett replikamål över SSH. Se [Parkoppla instanser](offsite-recovery.md#pairing) och [Skicka till en ZFS-server](zfs-datasets.md#replica-push).

## Integrationer {#integrations}

- **MCP-server**: nycklar för AI-assistenter, **Välj en klient** och varje nyckels logg. Se [MCP-server](mcp.md).
- **API-token**: namngivna token för skript och instrumentpaneler. Se [Token](api.md#tokens).
- **Tjänster**: **Home Assistant** över MQTT, **Hitta i nätverket** (mDNS som `bombvault.local`) och **Exponera /metrics** för Prometheus, med en **Scrape-token (valfritt)**. Se [Home Assistant](api.md#home-assistant) och [Hitta BombVault i nätverket](api.md#mdns).
- **Dashboard-widget**: en skrivskyddad token och en adress för vilken instrumentpanel som helst som visar en iframe, till exempel Homepage eller Heimdall, med bara den levande aktivitetsloggen. Se [Dashboard-widget](api.md#dashboard-widget).
- **Värd-SSH**: BombVaults publika nyckel och **Testa anslutning**, för VM- och ZFS-säkerhetskopior och Unraid-aviseringar. Se [guiden för VM-säkerhetskopiering](vm-backup-ssh-setup.md).

## Appar {#apps}

- **Android-app**: APK-filen för den version din server kör med en QR-kod, och F-Droid-listningen. Se [Android-appen](android.md#install).
- **ParleyPort**: relä-containern med sin Unraid-mall och Docker-kommandot som startar den.
- **BombVault Widget**: en bricka på Unraids Dashboard med BombVaults aktivitetslogg och nästa körning. Med en SSH-anslutning till värden installerar och tar kortet bort pluginet själv. Utan en sådan ger det dig `.plg`-adressen att installera under **Plugins, Install Plugin**.

## System {#system}

- **Exportera / importera inställningar**: hela din konfiguration som en portabel JSON-fil, med eller utan inloggningsuppgifterna. Importen visar först **Den här filen innehåller** och utelämnar länkar till lagringsmål som inte finns längre. **Kryptera exporter med age** förseglar de vanliga exporterna, flash-ZIP-filen och återställningskitet för de **age-mottagare** du listar. Se [Portabla inställningar](configuration.md#portable-settings-export-and-import).
- **Prestanda**: **CPU-trådar (0 = alla)**, **Gräns för cachestorlek (MB, 0 = obegränsat)** för restics cache under `/config`, och **Minnesgräns per säkerhetskopiering (MiB, 0 = ingen)**, en mjuk gräns som får en stor säkerhetskopiering att gå långsammare i stället för att dödas för att minnet tar slut.

## Info {#info}

- **Version**: BombVault, GlimStone, restic och rclone, med **Sök efter uppdateringar** och **Ny**, som räknar de ändringar du inte sett än och öppnar ett fönster som listar dem.
- **Hjälp**: **Manual** öppnar den här dokumentationen, och **Felrapport** laddar ner en redigerad ZIP-fil för ett GitHub-ärende. Den kräver ett inloggningslösenord.
- **Om BombVault**: projektet, dess licens och hur du når upphovsmannen.
