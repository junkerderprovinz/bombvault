# Konfiguration

Den här sidan täcker containerns miljövariabler, monteringarna som mallen tillhandahåller, VM-säkerhetskopiering över SSH och off-site-uppsättningen. Säkerhetskopieringens **repository-sökvägar** konfigureras inuti appen (Inställningar, Säkerhetskopiesökvägar), inte via miljövariabler.

## Miljövariabler

| Variabel | Obligatorisk | Beskrivning |
|---|---|---|
| `APP_KEY` | **Ja** | 32-byte hex-hemlighet (64 hex-tecken) som används för att härleda restic-repo-lösenordet. Generera med `openssl rand -hex 32`. Förvara den säkert: att förlora den gör krypterade säkerhetskopior oåterställbara. |
| `LIBVIRT_HOST` | För VM:ar | Unraid-värd nådd över SSH för VM-säkerhetskopiering (standard `host.docker.internal`; mallen förifyller en LAN-IP-platshållare). Använd din Unraid-LAN-IP, obligatorisk på ett anpassat `br0.x`-nätverk. |
| `LIBVIRT_SSH_PORT` | Nej | Värdens SSH-port för VM-säkerhetskopiering (standard `22`). |
| `LIBVIRT_SSH_USER` | Nej | SSH-användare på värden för VM-säkerhetskopiering (standard `root`). |
| `PORT` | Nej | HTTP-port (standard `3000`; används endast med `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Nej | HTTPS-port (standard `3443`; mallen publicerar den 1:1, så WebUI svarar på `https://<ip>:3443`). |
| `HTTP_ONLY` | Nej | Sätt `true` för att inaktivera den självsignerade HTTPS-lyssnaren och servera enbart vanlig HTTP (för användning bakom en TLS-terminerande reverse proxy). |
| `HOST_SOURCE_ROOT` | Nej | Värdsökvägen monterad som **Host Data** (standard `/mnt`). BombVault översätter de bind-monteringskällor Docker rapporterar till sökvägar under den här monteringen. Ändra endast om du monterade en annan värdrot. |
| `BOMBVAULT_SELF_CONTAINER` | Nej | Namnet på själva BombVault-containern, så att den aldrig säkerhetskopierar (och därmed stoppar) sig själv (standard `BombVault`; autodetekterad via värdnamnet vid bridge-nätverk). |
| `BACKUP_MAX_HOURS` | Nej | Maximalt antal väggklockstimmar en enskild säkerhetskopieringskörning får hålla sitt domänlås innan den tvingas avbrytas (ett skydd så att en fastnad körning inte kan blockera domänen för alltid). Tomt (standard) använder `48`. Höj det för mycket stora eller långsamma molnsäkerhetskopior (en körning som avbryts vid taket misslyckas med `context deadline exceeded`). Sätt `0` för att inaktivera taket helt. |
| `TZ` | Nej | Tidszon för schemaläggaren (till exempel `Europe/Berlin`). |

## Monteringar

Montera Docker-socketen, flashen (`/boot`) och **Host Data**-roten (`/mnt`) som visas i CA-mallen. Både säkerhetskopieringens *källor* och *mål* ligger under Host Data, och den monteras **rslave** så att en fjärresurs som monteras efter att containern startat (till exempel under `/mnt/remotes`) blir synlig utan en omstart.

Säkerhetskopieringens repository-sökvägar har standardvärdet `/mnt/user/bombvault/{container,vms,flash,config,files}`, skapade vid den första säkerhetskopieringen. Ändra platsen när som helst i **Inställningar, Säkerhetskopiesökvägar**.

!!! note "Värdintegrationskontroll"
    Öppna `/spike` i webbgränssnittet efter att containern startat. Den sonderar varje montering och CLI (Docker-socket, libvirt, restic, qemu-img, rclone) och rapporterar eventuella saknade delar.

## Säkerhetsmodell

!!! warning "Root-likvärdig kontroll över värden"
    Via Docker-socketen kan BombVault stoppa, ta bort och återskapa containrar och läsa/skriva appdata, och för VM-säkerhetskopiering loggar den in på värden över SSH (`qemu+ssh://`, root som standard) för att köra `virsh`. Vem som helst som kan nå dess webbgränssnitt har i praktiken root på värden.

- **Valfritt lösenordsskydd** (Inställningar, Säkerhet): ange ett lösenord för att kräva inloggning, rensa det för att inaktivera. Av som standard för användning på ett betrott LAN. Sessioner är signerade (HMAC härledd från `APP_KEY`) och att byta lösenord ogiltigförklarar dem; inloggningar är hastighetsbegränsade.
- Eftersom spärren är opt-in är hela gränssnittet och API:et (inklusive off-site-uppsättningen, manipulationstest-rutterna och återställningskitet) nåbara av vem som helst som kan nå porten när den är osatt. Aktivera spärren när off-site, oföränderliga säkerhetskopior eller kryptering används.
- Kör BombVault endast på ett betrott, icke-exponerat nätverk. För fjärråtkomst, placera den bakom en reverse proxy som lägger till autentisering och TLS. Svar bär baslinje-säkerhetsrubriker (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Med `HTTP_ONLY=true` förlorar sessionscookien sin `Secure`-flagga (den måste det, för att fungera över vanlig HTTP), så aktivera bara lösenordet bakom en TLS-terminerande proxy om konfidentialitet är viktig.
- VM-säkerhetskopieringens SSH-anslutning litar på värdnyckeln vid första anslutningen (TOFU) och pinnar den därefter. Verifiera värdens nyckel out-of-band om din container-till-värd-väg inte är betrodd.
- Säkerhetskopior krypteras av restic när kryptering är aktiverad (Inställningar; på som standard), med nyckeln härledd från `APP_KEY`.

## VM-säkerhetskopiering över SSH

BombVault säkerhetskopierar KVM/libvirt-VM:ar **utan att montera någon libvirt-sökväg**. Den kör `virsh` på värden över SSH (`qemu+ssh://`), så den kan aldrig påverka din värds VM Manager.

Snabbuppsättning:

1. **Inställningar, System, VM Backup over SSH:** kopiera den visade publika nyckeln.
2. Lägg till den i Unraids `/root/.ssh/authorized_keys` (även bevarad till flashen så att den överlever omstarter).
3. Klicka på **Testa anslutning**.

Mallen lägger till `--add-host=host.docker.internal:host-gateway` så att containern kan nå värden. Sätt `LIBVIRT_HOST` till din Unraid-LAN-IP om det namnet inte löses upp (till exempel när containern körs på ett anpassat `br0.x`-nätverk). Om du ändrade Unraids SSH-port, sätt `LIBVIRT_SSH_PORT` att matcha. **Live-ögonblicksbilder** behöver dessutom qemu-gästagenten i VM:en och disken på `/mnt/cache` (inte `/mnt/user`).

!!! important "Fullständig VM-uppsättnings- och nätverksguide"
    Den kompletta steg-för-steg-guiden (SSH-aktivering, beständig nyckelauktorisering, anpassat-nätverk- och VLAN-routning, metod per VM och felsökning på värdsidan) finns på [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) på GitHub.

## Off-site-uppsättning

Sätt upp en off-site-replik på fliken **Inställningar, Off-site**. Se [Off-site och återställning](offsite-recovery.md) för hela arbetsflödet (oföränderligt/append-only, manipulationstest och DR-övningar). I korthet:

- **Backender:** SMB/CIFS och NFS (montera resursen och peka en säkerhetskopiesökväg mot den), native restic-backender utan rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`), eller valfri rclone-fjärr (`rclone:<remote>:<bucket>/path`).
- **Molnuppgifter** lagras krypterade under Inställningar, Off-site, Molnuppgifter.
- **SSH-mål kräver inget installerat på den bortre sidan.** `sftp:` behöver bara en SSH-server. Lägg till den publika nyckeln från **Inställningar, System, VM Backup over SSH** (även på `/config/ssh/id_ed25519.pub`) i målanvändarens `~/.ssh/authorized_keys`.
- **Off-site-kopia:** BombVault replikerar nya ögonblicksbilder med `restic copy` på best-effort-basis. Det lokala repot förblir primärt. Varje domän har sitt eget off-site-schema, plus en **Replikera nu**-knapp.
- **Flera off-site-mål per domän:** varje domän kan replikera till flera off-site-mål samtidigt. Lägg till extra mål under Inställningar, Off-site, var och en med sitt eget repository, S3-lagringsklass, append-only-flagga, retention och tillväxtbudget; de replikerar alla enligt den domänens off-site-schema. En befintlig enskild off-site-uppsättning förs över som det första målet.
- **Retention per källa:** den lokala policyn finns under Inställningar, Sökvägar och lagring; off-site-policyn under Inställningar, Off-site (lämna den helt-noll för att aldrig autotrimma off-site-ögonblicksbilder).
- **Bandbreddsgränser:** begränsa restics uppladdnings-/nedladdningshastighet under Inställningar, Off-site.
- **Kall och arkivlagringsklass (S3):** för ett native S3-off-site-repo, välj en återställningsläsbar nivå (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone-fjärrar ställer in sin klass i rclone-konfigurationen.

## Portabla inställningar (exportera och importera) {#portable-settings-export-and-import}

Kortet **Exportera och importera inställningar** på Inställningar-sidan skriver hela din BombVault-konfiguration (domäninställningar, off-site-mål, scheman, retention, aviseringar) till en portabel JSON-fil som du kan importera på en annan instans, så att en flytt till en ny box eller kloning av en uppsättning inte innebär att allt måste matas in på nytt för hand. Import visar en förhandsgranskning och ber om bekräftelse, och den rör aldrig dina säkerhetskopieringsdata eller historik.

!!! warning "Exporten kan innehålla uppgifter"
    Du väljer om off-site- och aviseringsuppgifterna ska inkluderas i filen. Med uppgifter inkluderade är exporten lika känslig som ditt återställningskit, så förvara den på en säker plats. Utan dem innehåller filen endast icke-hemliga inställningar.
