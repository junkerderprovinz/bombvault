# Konfiguration

Den här sidan täcker containerns miljövariabler, monteringarna som mallen tillhandahåller, VM-säkerhetskopiering över SSH och off-site-uppsättningen. Var säkerhetskopiorna hamnar ställer du in inuti appen, under **Inställningar, Lagring**, inte via miljövariabler.

## Miljövariabler

| Variabel | Obligatorisk | Beskrivning |
|---|---|---|
| `APP_KEY` | **Ja** | 32-byte hex-hemlighet (64 hex-tecken) som används för att härleda restic-repo-lösenordet. Generera med `openssl rand -hex 32`. Förvara den säkert: att förlora den gör krypterade säkerhetskopior oåterställbara. |
| `LIBVIRT_HOST` | För VM:ar | Unraid-värd nådd över SSH för VM-säkerhetskopiering (standard `host.docker.internal`; mallen förifyller en LAN-IP-platshållare). Använd din Unraid-LAN-IP, obligatorisk på ett anpassat `br0.x`-nätverk. |
| `LIBVIRT_SSH_PORT` | Nej | Värdens SSH-port för VM-säkerhetskopiering (standard `22`). |
| `LIBVIRT_SSH_USER` | Nej | SSH-användare på värden för VM-säkerhetskopiering (standard `root`). |
| `LIBVIRT_URI` | Nej | Fullständig anslutnings-URI för libvirt, används **ordagrant** i stället för att bygga en från de tre `LIBVIRT_*`-variablerna ovan (som då ignoreras för anslutningssträngen). Inte satt som standard. Behövs på TrueNAS Scale, vars libvirtd lyssnar på en icke-standardsocket som den sammansatta strängformen inte kan uttrycka: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Se TrueNAS Scale-avsnittet i [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). |
| `PORT` | Nej | HTTP-port (standard `3000`; används endast med `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Nej | HTTPS-port (standard `3443`; mallen publicerar den 1:1, så WebUI svarar på `https://<ip>:3443`). |
| `HTTP_ONLY` | Nej | Sätt `true` för att inaktivera den självsignerade HTTPS-lyssnaren och servera enbart vanlig HTTP (för användning bakom en TLS-terminerande reverse proxy). |
| `TRUSTED_PROXY` | Nej | Kommaseparerade adresser eller CIDR-intervall för den omvända proxyn framför BombVault (till exempel `192.168.20.11` eller `10.0.0.0/8`). Endast från dessa hopp tros `X-Forwarded-For`, och inloggningsbromsen räknar då misslyckanden per verklig klient i stället för att lägga alla bakom proxyn i samma hink. Osatt (standard) betyder att ingen tros: en villkorslöst trodd rubrik skulle låta vem som helst välja sin egen hink. |
| `HOST_SOURCE_ROOT` | Nej | Värdsökvägen monterad som **Host Data** (standard `/mnt`). BombVault översätter de bind-monteringskällor Docker rapporterar till sökvägar under den här monteringen. Ändra endast om du monterade en annan värdrot. |
| `DATA_ROOT_SEGMENTS` | Nej | Kommaseparerade sökvägssegmentnamn som markerar en bind-monteringskälla som säkerhetskopieringsdata (standard `appdata`, vilket matchar Unraids `/mnt/user/appdata/<container>`-konvention). En containers bind-montering väljs automatiskt för säkerhetskopiering när NÅGOT listat segment förekommer som ett fullständigt sökvägssegment i dess värdkälla, till exempel plockar `DATA_ROOT_SEGMENTS=appdata,config` även upp en `.../config`-bindning. Se [Identifiering av säkerhetskopieringskällor](#backup-source-detection) för de andra, alltid aktiva sätten en containers datamapp hittas på. |
| `PLATFORM` | Nej | Tvingar vilken plattform BombVault uppfattar sig själv som att köra på, i stället för att identifiera den automatiskt: `unraid`, `generic` eller `truenas` (standard inte satt: identifierar Unraid automatiskt genom att sondera efter dess `dockerMan`-markör under flash-monteringen, annars `generic`; ett okänt värde faller också tillbaka till `generic`, vilket loggas). Sätt den explicit på en generisk Docker-värd eller TrueNAS Scale i stället för att förlita dig på den Unraid-specifika autosonderingen, vilket den generiska compose-filen gör. Ändrar reservkonventionen för appdata, standardmålen för återställning mellan instanser, och om de Unraid-specifika stegen för aviseringar och kompanjonsplugin ens försöks (se `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Nej | Namnet på själva BombVault-containern, så att den aldrig säkerhetskopierar (och därmed stoppar) sig själv. |
| `BACKUP_MAX_HOURS` | Nej | Maximalt antal väggklockstimmar en enskild säkerhetskopieringskörning får hålla sitt domänlås innan den tvingas avbrytas (ett skydd så att en fastnad körning inte kan blockera domänen för alltid). Tomt (standard) använder `48`. Höj det för mycket stora eller långsamma molnsäkerhetskopior (en körning som avbryts vid taket misslyckas med `context deadline exceeded`). Sätt `0` för att inaktivera taket helt. |
| `TZ` | Nej | Tidszon för schemaläggaren (till exempel `Europe/Berlin`). **Om den inte anges körs alla scheman i UTC**: ett schema satt till 02:30 startar då 02:30 UTC och inte enligt lokal tid. På Unraid ställer du aldrig in detta själv: systemet skickar sin egen tidszon till varje container. |

## Monteringar

Montera Docker-socketen, flashen (`/boot`) och **Host Data**-roten (`/mnt`) som visas i CA-mallen. Både säkerhetskopieringens *källor* och *mål* ligger under Host Data, och den monteras **slave** så att en fjärresurs som monteras efter att containern startat (till exempel under `/mnt/remotes`) blir synlig utan en omstart.

En ny installation sparar varje domän på lagringsplatsen **Unraid**, under `/mnt/user/bombvault` med en mapp per domän (`container`, `vms`, `flash`, `config`, `files`), som skapas vid den första säkerhetskopieringen. Fler lagringsplatser, lokala eller fjärranslutna, lägger du till under **Inställningar, Lagring**; se [Lagringsplatser](storage-places.md).

!!! note "Värdintegrationskontroll"
    Öppna `/spike` i webbgränssnittet efter att containern startat. Den sonderar varje montering och CLI (Docker-socket, libvirt, restic, qemu-img, rclone) och rapporterar eventuella saknade delar.

## Igenkänning av säkerhetskopieringens källor {#backup-source-detection}

För varje container väljer BombVault själv vilka bind-monteringar och namngivna volymer som ska säkerhetskopieras. En sökväg tas med så snart någon av följande punkter gäller (resultatet kan alltid skrivas över per container under dess **Säkerhetskopieringssökvägar**):

- **Träff på ett datarot-segment:** bindens värdkälla innehåller något av segmenten i `DATA_ROOT_SEGMENTS` som en fullständig sökvägsdel (som standard endast `appdata`).
- **Namngivna Docker-volymer** tas alltid med, eftersom de saknar en slängbar motsvarighet och det därmed inte finns något att filtrera bort, **men bara när volymens verkliga lagringssökväg på värden själv går att nå genom Host Data-monteringen**, precis som varje annan värdsökväg BombVault säkerhetskopierar. Standarddrivrutinen för lokala volymer lägger en volym under demonens egen datarot, alltså `/var/lib/docker/volumes/<namn>/_data` om inget ändrats (kontrollera med `docker info -f '{{.DockerRootDir}}'`). Den platsen omfattas INTE av den smala Host Data-monteringen med en enda katalog som den generiska `docker-compose.yml` använder som standard. En onåbar volym hoppas tyst över, det är inget fel. För att verkligen säkerhetskopiera namngivna volymer på en generisk värd, rikta Host Data (och `HOST_SOURCE_ROOT`) mot en gemensam överordnad katalog som även täcker Dockers datarot: avvägningen står i Host Data-kommentaren i compose-filen (Unraid kringgår detta genom att av samma skäl montera hela `/mnt`, sin egen allmängiltiga konvention på översta nivån).
- **Projektkatalog för Docker Compose:** bär containern den vanliga etiketten `com.docker.compose.project.working_dir` (sätts automatiskt av `docker compose up`) läggs även den katalogen till, oavsett om någon bind matchade ett datarot-segment.
- **Åsidosättning med etiketten `bombvault.data`:** sätt etiketten `bombvault.data=true` på en container för att ta med ALLA dess bind-monteringar, för en uppställning som ingen av konventionerna ovan fångar (till exempel en enda bind `/srv/plex/config` utan Compose-projekt). Varje icke-tomt värde utom `false` räknas som sant; en saknad etikett eller `bombvault.data=false` ändrar ingenting.

## Säkerhetsmodell

!!! warning "Root-likvärdig kontroll över värden"
    Via Docker-socketen kan BombVault stoppa, ta bort och återskapa containrar och läsa/skriva appdata, och för VM-säkerhetskopiering loggar den in på värden över SSH (`qemu+ssh://`, root som standard) för att köra `virsh`. Vem som helst som kan nå dess webbgränssnitt har i praktiken root på värden.

- **Valfritt lösenordsskydd** (Inställningar, Säkerhet): ange ett lösenord för att kräva inloggning, rensa det för att stänga av. Av som standard för användning i ett betrott LAN. Lösenordet lagras med Argon2id över ett värde pepprat med `APP_KEY`, så en kopierad `/config` är värdelös utan nyckeln och långsam att angripa med den. Ett nytt lösenord kräver minst 12 tecken; ett befintligt kortare fungerar tills det ändras. Sessioner är signerade (HMAC härledd ur `APP_KEY`) och en lösenordsändring ogiltigförklarar dem; inloggningar begränsas till fem misslyckanden per minut och klient.
- **Tvåfaktorsautentisering** (Inställningar): en tidskod från en autentiseringsapp utöver lösenordet, plus åtta engångsåterställningskoder som lämnas ut en gång när den slås på. Den delade hemligheten lagras krypterad med `APP_KEY`, och att stänga av den igen kräver en aktuell kod.
- Eftersom spärren är opt-in är hela gränssnittet och API:et (inklusive off-site-uppsättningen, manipulationstest-rutterna och återställningskitet) nåbara av vem som helst som kan nå porten när den är osatt. Aktivera spärren när off-site, oföränderliga säkerhetskopior eller kryptering används.
- Kör BombVault endast på ett betrott, icke-exponerat nätverk. För fjärråtkomst, placera den bakom en reverse proxy som lägger till autentisering och TLS. Svar bär baslinje-säkerhetsrubriker (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Bakom en omvänd proxy bär varje begäran proxyns adress, så utan `TRUSTED_PROXY` räknar inloggningsbromsen alla klienter i samma hink och en angripares misslyckanden låser ute även dig. Ange proxyn i `TRUSTED_PROXY` för att få tillbaka räkning per klient.
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

Off-site-kopior går till lagringsplatser. Lägg till lagringsplatsen under **Inställningar, Lagring** med **Lägg till lagringsplats**, och kryssa sedan i den under **Kopierad till** på domänens rad. [Lagringsplatser](storage-places.md) täcker alla anslutningstyper, och [Off-site och återställning](offsite-recovery.md) täcker append-only, manipulationstest och DR-övningar. I korthet:

- **Anslutningstyper:** en mapp på den här Unraid-servern eller en NAS-utdelning monterad under `/mnt/remotes`, S3-lagring (Backblaze B2 och de andra molnleverantörerna, eller en självhostad tjänst som MinIO eller Garage), rest-server, SFTP inklusive en Hetzner Storage Box, WebDAV för Nextcloud, ownCloud och OpenCloud, Azure Blob och valfri rclone-fjärr. Backblaze B2 behöver bara nyckeln: BombVault läser av bucketen och S3-slutpunkten ur den.
- **Inloggningsuppgifter** lagras krypterade tillsammans med lagringsplatsen de hör till. De uppsättningar av inloggningsuppgifter som hämtningskällor använder finns på fliken **Hämtning** på sidan Instanser.
- **SSH-mål kräver inget installerat på den bortre sidan.** En SFTP-lagringsplats behöver bara en SSH-server. Lägg till den publika nyckeln som visas i SFTP-formuläret (även under **Inställningar, System, VM-säkerhetskopia över SSH** och på `/config/ssh/id_ed25519.pub`) i målanvändarens `~/.ssh/authorized_keys`.
- **Off-site-kopia:** BombVault kopierar nya ögonblicksbilder med `restic copy` på best-effort-basis, utöver lagringsplatsen som domänen sparas på. Varje domän har sitt eget kopieringsschema under Inställningar, Scheman, plus **Kopiera nu** på sin rad.
- **Flera kopieringsplatser per domän:** kryssa i så många lagringsplatser du vill under **Kopierad till**; var och en kopierar enligt domänens schema.
- **Retention, gränser, lagringsklass och tillväxtbudget hör till lagringsplatsen** och ställs in i dess detaljer. Retentionen för en lagringsplats gäller för varje arkiv på den, så en off-site-lagringsplats kan behålla kopior längre som ett arkiv; en lagringsplats där varje regel står på noll trimmar aldrig.
- **Kall och arkivlagringsklass (S3):** för en S3-lagringsplats, välj en återställningsläsbar nivå (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). rclone-fjärrar ställer in sin klass i rclone-konfigurationen.
- **En domän sparad på en fjärransluten lagringsplats:** se [En domän sparad på en fjärransluten lagringsplats](offsite-recovery.md#remote-primary-repositories).

## Portabla inställningar (exportera och importera) {#portable-settings-export-and-import}

Kortet **Exportera och importera inställningar** på Inställningar-sidan skriver hela din BombVault-konfiguration (domäninställningar, lagringsplatser, scheman, aviseringar) till en portabel JSON-fil som du kan importera på en annan instans, så att en flytt till en ny box eller kloning av en uppsättning inte innebär att allt måste matas in på nytt för hand. Import visar en förhandsgranskning och ber om bekräftelse, och den rör aldrig dina säkerhetskopieringsdata eller historik. Förhandsgranskningen räknar lagringsplatserna i filen och, med uppgifterna, autentiseringsuppsättningarna; för en äldre fil utan lagringsplatser bygger BombVault dem från de importerade inställningarna.

!!! warning "Exporten kan innehålla uppgifter"
    Du väljer om inloggningsuppgifterna för dina lagringsplatser och aviseringar ska inkluderas i filen. Med uppgifter inkluderade är exporten lika känslig som ditt återställningskit, så förvara den på en säker plats. Utan dem innehåller filen endast icke-hemliga inställningar.
