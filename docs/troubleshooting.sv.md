# Felsökning

En kort FAQ. För den fullständiga felsökningstabellen för VM-över-SSH på värdsidan (permission-denied, host-key-verifiering, saknade mallvariabler och mer), se [guiden för VM-säkerhetskopiering över SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub.

## Något är inte korrekt inkopplat

Öppna `/spike` i webbgränssnittet. Värdintegrationskontrollen sonderar varje montering och CLI (Docker-socket, libvirt, restic, qemu-img, rclone) och rapporterar eventuella saknade delar. Börja här innan du antar att det är ett fel: en saknad montering eller en onåbar värd dyker upp omedelbart.

## Jag kan inte nå webbgränssnittet

BombVault serverar HTTPS direkt ur lådan på port `3443` (självsignerat certifikat), så öppna `https://<din-unraid-ip>:3443`. Godkänn varningen om det självsignerade certifikatet, eller placera BombVault bakom en reverse proxy med ditt eget certifikat. Om du kör med `HTTP_ONLY=true` serverar den vanlig HTTP på port `3000` istället (avsedd för användning bakom en TLS-terminerande proxy).

## Jag förlorade min APP_KEY

`APP_KEY` härleder restic-repositoriets lösenord. Utan den (och utan återställningskitet för krypteringsnyckeln) kan krypterade säkerhetskopior inte återställas. Det är därför Översikten tjatar på dig att ladda ner återställningskitet. Se [Off-site och återställning](offsite-recovery.md). Generera en nyckel med `openssl rand -hex 32` och förvara den bort från servern innan du förlitar dig på någon säkerhetskopia.

## VM-säkerhetskopiering ansluter inte

VM-säkerhetskopiering pratar med libvirt över SSH, aldrig en montering.

- Bekräfta att SSH är aktiverat på värden och att BombVaults publika nyckel är auktoriserad i `/root/.ssh/authorized_keys` (Inställningar, System, VM Backup over SSH visar nyckeln och en **Testa anslutning**-knapp).
- På ett anpassat `br0.x`-nätverk, sätt `LIBVIRT_HOST` till din Unraid-LAN-IP (containern kan inte nå värden via `host.docker.internal` där). Aktivera **Inställningar, Docker, Host access to custom networks**.
- Om du ändrade Unraids SSH-port, sätt `LIBVIRT_SSH_PORT` att matcha.
- Fullständig steg-för-steg-diagnos (nåbarhetstest, VLAN-routning, `Permission denied (publickey)`, `Host key verification failed`) finns i [guiden för VM-säkerhetskopiering över SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## En live-VM-ögonblicksbild kördes inte

Live-ögonblicksbilder behöver qemu-gästagenten installerad i VM:en och disken på `/mnt/cache` (eller `/mnt/diskX`), inte `/mnt/user`. På en avstängd VM faller live automatiskt tillbaka till mjuk. En mjuk säkerhetskopiering stänger av VM:en, säkerhetskopierar diskarna och startar sedan om den, så den är alltid konsekvent.

## En säkerhetskopiering misslyckades med "repository is already locked"

Detta är oftast ett övergivet restic-lås som lämnats kvar när containern uppdaterades eller startades om mitt i en operation. BombVault upptäcker ett bevisligen övergivet lås, tvingar bort det och gör om en gång, automatiskt. Om det kvarstår, använd **Inställningar, Integritet och underhåll, Lås upp** för den drabbade domänen för att rensa ett fastnat lås för hand. Ett äkta problem dyker fortfarande upp istället för att döljas.

## Min off-site-kopia hände inte efter en säkerhetskopiering

Off-site-replikering är best-effort by design, så att en off-site-hicka aldrig misslyckar den lokala säkerhetskopian. Kontrollera off-site-schemat för den domänen (Inställningar, Scheman): ett tomt schema replikerar efter varje lokal säkerhetskopiering, medan en kadens skickar mer sällan. Använd **Replikera nu** på Off-site-fliken för en körning på begäran, och håll koll på replikeringsindikatorn på Översikten.

## En återställning avbröts innan den startade

Innan något stoppas eller tas bort kör återställningen en konfliktkontroll före körning: den verifierar att containerns statiska IP och publicerade värdportar är lediga. Om en annan container redan håller en av dem avbryter den med ett tydligt, åtgärdbart meddelande istället för att lämna en halvfärdig återställning. Frigör den konfliktande porten eller IP:n och försök igen.

## En vanlig export misslyckades istället för att skriva en fil

Om age-kryptering är på (Inställningar) men ingen giltig mottagare är satt misslyckas en export med ett tydligt fel istället för att skriva klartext. Lägg till en giltig mottagare (en age-publik nyckel eller en SSH-publik nyckel), eller stäng av kryptering om du avser att exporten ska vara klartext. Se [Funktioner](features.md).

## En databasdump misslyckades

En misslyckad dump fäller aldrig säkerhetskopian omkring den; den bokförs som en egen misslyckad körning, och orsaken säger vad som ska åtgärdas.

- **Inloggning nekad.** Dumpen loggar in med containerns egna lösenordsvariabler (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` eller deras `_FILE`-varianter). Kontrollera dem på databascontainern. En `_FILE`-variabel som pekar på en hemlighet containerns egen användare inte får läsa misslyckas på samma sätt.
- **Saknade rättigheter.** Med ett slumpmässigt root-lösenord kan dumpen bara logga in som appanvändaren och innehåller därmed bara den ena databasen, och MySQL 8.4 och senare kan neka den helt. Ge containern ett riktigt root-lösenord, eller stäng av dumpen för den.
- **Systemtabellerna behöver uppgraderas.** MariaDB vägrar dumpas när dess systemtabeller kommer från en äldre version (fel 1558). Lägg till variabeln `MARIADB_AUTO_UPGRADE=1` och starta om containern, eller kör `mariadb-upgrade` inuti den en gång.
- **Inget dumpverktyg.** En slimmad eller egenbyggd avbild utan `pg_dump`, `mysqldump` eller `mariadb-dump` går inte att dumpa. Använd den officiella avbilden, eller stäng av dumpen.
- **En tidsgräns.** En dump får `DB_DUMP_MAX_HOURS` (6 som standard), säkerhetskopian omkring får `BACKUP_MAX_HOURS`, och en dump som slutar göra framsteg kapas efter `BACKUP_STALL_HOURS`. Det sista beror oftast på ett lås som applikationen håller. Höj den gräns som slog till, eller dumpa medan applikationen är lugn.
- **Containern är pausad eller startar om.** Dumpen pratar med servern medan den kör. Om containern startar om gång på gång säger dess egen logg varför.
- **En skadad dump gick inte att ta bort.** En dump som BombVault inte kunde slutföra raderas igen. När den raderingen misslyckas står dumpen kvar i listan märkt som skadad, och du kan ta bort den där.

## En import misslyckades

En import stoppar containern, flyttar undan dess datamapp och låter avbilden skapa en tom i stället. Misslyckas ett steg före själva importen läggs den gamla mappen tillbaka av sig själv. Misslyckas importen behåller containern den färska mappen, och den gamla ligger kvar bredvid som `<datamapp>.bombvault-before-import-<tidsstämpel>`; körningens felmeddelande namnger den exakta sökvägen.

Så lägger du tillbaka den för hand: stoppa containern, byt namn på den nuvarande datamappen så att den är ur vägen, byt tillbaka den bevarade mappen till det ursprungliga namnet och starta containern. På Unraid gör filhanteraren under fliken Shares detta.

## En säkerhetskopia av en ZFS-datauppsättning misslyckades eller hoppade över en uppsättning {#zfs-datasets}

Varje problem har en orsakskod inom hakparenteser, och sidan [ZFS-datauppsättningar](zfs-datasets.md#reason-codes) listar alla med åtgärden. De tre vanligaste:

- **`snapshot-loop`**: ögonblicksbilden nådde inte BombVault eftersom Host Data inte skickar vidare nya monteringar. Redigera containern, sätt Access Mode för Host Data till Read/Write - Slave och starta om BombVault.
- **`key-not-loaded`**: en krypterad uppsättning vars nyckel inte är laddad hoppas över. Ladda nyckeln med `zfs load-key` och montera uppsättningen; nästa säkerhetskopia tar med den.
- **`ssh-auth`**: servern avvisade BombVaults nyckel. Anslutningskortet på ZFS-sidan visar kommandot som godkänner den; kör det en gång på servern.

## Ett objekt står kvar på "Lär sig N/10"

De flesta avvikelsekontroller börjar efter 10 lyckade säkerhetskopior av ett objekt, och räkningen börjar om efter **Markera som väntad** och efter att objektets urval har ändrats. Ett objekt utan schema lär sig inte, och en container utan appdata har inget att lära sig av, vilket dess märke också säger.

## Gallringen har slutat ta bort gamla säkerhetskopior för ett objekt

En öppen kritisk avvikelse håller kvar dem: objektets källa är nästan tom, har krympt kraftigt, eller en säkerhetskopia har sparat det mesta av datan på nytt. Öppna avvikelsen från märket på objektet. Om data saknas eller har krypterats, återställ först från den länkade senaste bra säkerhetskopian. Kvittera sedan avvikelsen, eller markera den som väntad om ändringen var din, så gallrar nästa körning som vanligt. Förhandsvisningen av gallringen markerar ett sådant objekt som behållet.

## Manuell rensning säger att vissa objekt behölls

Samma orsak: rensningen låter de gamla säkerhetskopiorna för ett objekt med en sådan avvikelse vara och nämner objektet i sitt meddelande. Allt annat rensas som vanligt.

## Historikimporten säger att ett repository inte kunde läsas

Efter uppgraderingen läser BombVault en gång storleken på tidigare säkerhetskopior ur varje repository. Ett repository som inte gick att nå då, till exempel ett externt mål som låg nere eller en share som inte var monterad, listas i kortet **Avvikelser** under **Inställningar, Integritet** och prövas igen en gång om dagen. Under tiden lär sig dess objekt av nya säkerhetskopior.

## Varningen om diskutrymme stämmer inte med Unraids instrumentpanel

På Unraids användarshare (`/mnt/user`) är det lediga utrymmet hela arrayens, inte en enskild disks. Fjärrepositorier mäts bara via rclone-fjärrar som rapporterar sitt lediga utrymme; S3-, B2-, REST- och SFTP-repositorier saknar uppgift och listas som ej uppmätta i kortet **Avvikelser**.

## En AI-assistent kan inte ansluta

Sidan [MCP-server](mcp.md#troubleshooting) visar vad varje statuskod och varje nekande från MCP-slutpunkten betyder och vad du kan göra åt det.

## Containern startar om hela tiden eller ser osund ut

BombVault rapporterar frisk/osund från sin egen `/api/health`. Ett auto-heal-verktyg (som Autoheal) kan starta om den automatiskt om motorn någonsin skulle kärva. Kontrollera containerloggen och `/spike`-rapporten för den underliggande orsaken.

## Fortfarande fast?

- Läs de fullständiga sidorna [Konfiguration](configuration.md) och [Off-site och återställning](offsite-recovery.md).
- Fråga på [Unraid-supporttråden](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Öppna ett [GitHub-ärende](https://github.com/junkerderprovinz/bombvault/issues).
