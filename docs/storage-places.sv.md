# Lagringsplatser

En lagringsplats är ett ställe där BombVault förvarar säkerhetskopior: en mapp på den här Unraid-servern, en utdelning på en NAS, en bucket hos en molnleverantör, en rest-server, ett SFTP-konto eller en Nextcloud. Du ansluter varje lagringsplats en gång, under **Inställningar, Lagring**, och dess inloggningsuppgifter, retention, skydd och uppgiften om var den står hör till den. De fem domänerna (containrar, VM:ar, flashen, BombVaults egen konfiguration och filuppsättningar) väljer sedan bland lagringsplatserna: var varje domän sparas och vart den kopieras.

## Lägga till en lagringsplats {#add-a-place}

**Lägg till lagringsplats** öppnar ett fönster med en ruta per leverantör, i tre grupper: molnlagring, självhostade tjänster samt NAS-enheter och den här servern.

1. Välj en ruta och fyll i dess formulär. Ögonknappen visar en hemlighet du har skrivit in.
2. **Testa anslutning** kontrollerar lagringsplatsen och skapar ingenting. För varje domäns mapp talar testet om vad det hittade: att mappen är tom eller inte finns än, att den redan innehåller ett restic-arkiv, eller felet som stoppade testet.
3. Ge lagringsplatsen ett namn; leverantörens namn är redan ifyllt. För en enhet du själv driver svarar du på **Var står enheten?**. Molnleverantörer står alltid på en annan plats, och en mapp på den här Unraid-servern står alltid här.
4. **Lägg till** sparar lagringsplatsen.

En ny lagringsplats används inte av någon domän än. Välj den under **Sparad i** eller **Kopierad till** på [kortet Domäner](#domains), eller på ett objekts kort för bara det objektet.

## Mappar {#folders}

En lagringsplats har en mapp per domän: `container`, `vms`, `flash`, `config` och `files`, samma namn som standardplatserna för säkerhetskopior använder. Mapparna listas i lagringsplatsens detaljer och kan byta namn där (se [Ändra en adress](#addresses)). En domän utan mapp på en lagringsplats kan inte välja den lagringsplatsen.

När en domän använder en lagringsplats i båda rollerna får den andra rollen ett suffix och den första behåller sin mapp. En lagringsplats som redan tar emot en domäns kopior sparar de objekt som skickas direkt dit i `<folder>-direct`; en lagringsplats som redan sparar en domän tar emot dess kopior i `<folder>-copies`.

Vissa lagringsplatser är själva ett restic-arkiv: en adress som redan innehöll ett arkiv när lagringsplatsen lades till, ett namngivet arkiv från en befintlig uppsättning, eller ett kopieringsmål i roten av en bucket. En sådan lagringsplats har inga mappar, alla domäner delar dess enda arkiv, och den kan inte ta en andra roll. För att spara mer hos samma leverantör ansluter du en annan bucket eller mapp som en egen lagringsplats.

## Lagringsplatsens detaljer {#details}

Varje lagringsplats är en rad med sin leverantör, vad den används till och dess senaste test eller kopia. **Testa** kontrollerar varje adress på lagringsplatsen, och **Detaljer** öppnar dess inställningar. Varje ändring i detaljerna sparas direkt när du gör den.

- **Allmänt**: namnet, omkopplaren som slår på och av lagringsplatsen, adressen och, för en enhet du själv driver, **Var står enheten?** (se [Utanför lokalerna](#off-the-premises)).
- **Kvarhållning**: keep-last, daglig, veckovis och månadsvis, för varje arkiv på lagringsplatsen. En ny lagringsplats börjar med standardreglerna; en lagringsplats där varje regel står på noll trimmar aldrig.
- **Skydd**: omkopplaren **Append-only**. Den bortre sidan måste upprätthålla append-only; med omkopplaren på rensar eller raderar BombVault aldrig något där. På en rest-server med append-only på kör **Testa append-only** manipulationstestet mot varje domänsökväg, påslagen kopia och arkiv på lagringsplatsen och visar ett svar för hela lagringsplatsen, *radering nekad* eller *radering tillåten* (se [Off-site och återställning](offsite-recovery.md)). Bara fjärranslutna lagringsplatser har det här avsnittet, eftersom ingenting på den här maskinen kan hindra att ett lokalt arkiv raderas.
- **Åtkomst**: inloggningsuppgifterna och, för S3, lagringsklassen. En lagringsplats som använder de gemensamma inloggningsuppgifterna får en egen uppsättning vid den första ändringen. Ett direkt arkiv på lagringsplatsen som de nya uppgifterna inte kan öppna behåller de gamla, och svaret säger det. Mapp-, SFTP- och rclone-lagringsplatser har inget sådant avsnitt.
- **Gränser**: uppladdnings- och nedladdningshastigheten och tillväxtbudgeten.
- **Mappar**: en omkopplare per domän, med namnet på dess mapp. En domän som är avstängd här kan inte välja lagringsplatsen.

Att sänka retentionen frågar först och säger hur många objekt det berör; att stänga av append-only frågar först och säger hur många arkiv på lagringsplatsen som förlorar det. Att stänga av en lagringsplats stänger av varje arkiv på den; en lagringsplats som en domän sparas på kan inte stängas av.

## Kortet Domäner {#domains}

Kortet har en rad per domän, med dess schema, var den sparas, vart den kopieras och dess undantag.

- **Sparad i**: så länge domänens säkerhetskopiesökväg inte innehåller några säkerhetskopior blir den valda lagringsplatsen domänens hem, och sökvägen flyttas dit. När den innehåller säkerhetskopior blir valet för containrar, VM:ar och filuppsättningar standard för nya objekt, som tar det vid sin första säkerhetskopiering; objekt som redan har säkerhetskopior stannar där de är, eftersom BombVault aldrig flyttar en säkerhetskopia. För flashen och BombVaults egen konfiguration flyttas hemmet, och de säkerhetskopior som redan skrivits blir kvar på den gamla lagringsplatsen.
- **Kopierad till**: ett chip per lagringsplats som kan ta emot domänens kopior. Att kryssa i ett chip gör lagringsplatsen till ett kopieringsmål för domänen; första gången säger BombVault i förväg hur många objekt och ögonblicksbilder och hur mycket data den första körningen skickar. Att kryssa ur stoppar nya kopior: kopiorna som redan finns där blir kvar och åldras enligt lagringsplatsens retention, och objekt med ett eget val fortsätter kopiera dit. Att kryssa ur det sista chippet stoppar alla kopior, även till lagringsplatser som läggs till senare, tills ett chip kryssas i igen. En avstängd lagringsplats visas som ett nedtonat chip och kan inte väljas.
- **Undantag**: objekten med ett eget val, som en lista med länkar till deras kort.
- **Kopiera nu** kör domänens kopior direkt.

En domän som pausats efter en ombyggnad via Identifiera visar pausen på sin rad, med **Bekräfta standard** (se [Placering per objekt](offsite-recovery.md#placement)).

## Ändra en adress {#addresses}

En domäns mapp kan ändras i lagringsplatsens detaljer, och det kan även adressen till en lokal lagringsplats, till exempel efter att ett arkiv har flyttats till en annan disk för hand. BombVault testar varje adress som ändringen berör och godtar den när varje ny adress är tom och ingenting var sparat på den gamla, eller när varje ny adress innehåller samma restic-arkiv som den gamla. Allt annat avvisas, med antalet säkerhetskopior som fortfarande ligger på den gamla adressen. En fjärransluten lagringsplats behåller sin adress; vill du säkerhetskopiera någon annanstans ansluter du den adressen som en egen lagringsplats.

BombVault bygger listan över lagringsplatser ur sin egen databas och listar aldrig ett fjärrarkiv för att fylla den; testet körs bara när du ändrar något.

## Ta bort en lagringsplats {#remove}

En lagringsplats kan bara tas bort så länge ingenting använder den: ingen domän sparas där, ingen standard pekar på den, inget objekt sparas där och inget direkt arkiv på den innehåller objekt. Annars listar avvisningen vad som håller kvar den. När den tas bort följer dess kopieringsmål med, liksom dess egna inloggningsuppgifter om inte en hämtningskälla eller en annan lagringsplats använder dem. Ingenting raderas i själva lagringen, och bekräftelsen säger hur många kopior som blir kvar där.

## Utan lagringsplats {#without-a-place}

En adress som inte passar formen lagringsplats plus mapp fortsätter att fungera och listas under **Utan lagringsplats**, med sin adress. Bland dem finns native `b2:`-, `gs:`- och `swift:`-adresser. **Koppla till lagringsplats** kopplar en sådan rad till en lagringsplats, efter samma test som vid [ändring av en adress](#addresses). Ett kopieringsmål utan lagringsplats nämns också på domänens rad, bredvid chippen, och fortsätter att kopiera. En fjärrad där har sin egen **Append-only**-omkopplare, och att stänga av den frågar först med antalet objekt som har säkerhetskopior på den adressen. Ett direkt arkiv följer omkopplaren för sitt mål.

## Utanför lokalerna {#off-the-premises}

**Var står enheten?** har två svar: **Här i huset** och **På en annan plats**. En kopia räknas som en egen plats, för 3-2-1-raden på korten och för Översiktens off-site-kontroller, bara när dess lagringsplats står på en annan plats. En andra disk eller en NAS i samma hus är en andra kopia, inte en andra plats. Svaret ändrar ingen kopia. Molnleverantörer står alltid på en annan plats och en mapp på den här Unraid-servern alltid här, så formuläret frågar inte efter dem; för alla andra lagringsplatser ändrar du svaret i detaljerna. En lagringsplats på en annan plats bär märket **Annan plats** på sin rad.

## Anslutningstyper

### Mapp på den här Unraid-servern eller en NAS {#kind-local}

Adressen är en sökväg under `/mnt`, skriven utan `/mnt`, till exempel `user/bombvault`, och varje domäns mapp ligger under den: `user/bombvault/container`.

- **Mapp på den här Unraid-servern** väljer bland utdelningarna, diskarna och poolerna.
- **Synology**, **QNAP**, **TrueNAS**, **En annan Unraid-server** och **Annan utdelning** väljer från `/mnt/remotes`. Montera utdelningen i Unraid först, till exempel med pluginet Unassigned Devices. Host Data måste vara monterad Read/Write - Slave, annars förblir en utdelning som monteras efter att BombVault startat osynlig tills en omstart (se [Konfiguration](configuration.md)).

Mappväljaren skapar en mapp där den står med **Ny mapp**. Testet kontrollerar att mappen är tom eller saknas och att BombVault kan skriva där.

### S3 {#kind-s3}

Adressen är `s3:https://<endpoint>/<bucket>/<path>`, till exempel `s3:https://s3.eu-central-003.backblazeb2.com/tower-backups/bombvault`.

- **Backblaze B2** behöver bara nyckel-ID och applikationsnyckel. BombVault frågar B2 vilken bucket, S3-slutpunkt och mapp nyckeln är begränsad till och bygger adressen utifrån det. En nyckel som når alla buckets erbjuder sina buckets att välja bland.
- **Amazon S3**, **Cloudflare R2**, **Wasabi**, **Hetzner Object Storage**, **Storj**, **IDrive e2**, **Scaleway**, **OVHcloud**, **DigitalOcean Spaces**, **IONOS**, **Contabo**, **Exoscale** och **Vultr** frågar efter nyckeln och, där leverantören behöver det, efter region, konto-ID eller slutpunkt. BombVault fyller i slutpunkten och listar dina buckets när nyckeln får lista dem; annars skriver du in bucketens namn.
- **Google Cloud Storage** nås via sitt S3-gränssnitt med en HMAC-nyckel, som du skapar i inställningarna för Cloud Storage under Interoperability. En fil för ett tjänstkonto fungerar inte här.
- **MinIO**, **SeaweedFS**, **Garage**, **Ceph**, **JuiceFS**, **RustFS**, **Versity S3 Gateway** och **Annan S3-tjänst** tar tjänstens adress och en nyckel.

Lagringsklassen ställs in i lagringsplatsens detaljer, begränsad till nivåer som en återställning kan läsa utan upptining.

### rest-server {#kind-rest}

Adressen är `rest:<url>/<user>`, till exempel `rest:https://nas.lan:8000/tower`. Formuläret frågar efter serverns adress, en användare och ett lösenord. Med `--private-repos` får en användare bara nå sökvägar som börjar med användarens eget namn, så BombVault sätter användaren först om du inte skriver en annan sökväg. När servern vägrar en sökväg utanför användarens egen säger felet det.

Formuläret för rest-server har ett recept, färdigt att klistra in, för en rest-server i append-only-läge med en användare för den här BombVault-instansen. **Visa recept** skapar ett lösenord, som visas en gång, och ger en `docker run`-rad, en compose-fil och en Unraid-mall, var och en med `htpasswd`-raden som ska in på servern; användaren och lösenordet går direkt in i formuläret.

**Ett annat BombVault** listar, ovanför sina egna fält, de öppna erbjudanden som andra instanser har skickat via Flotta. Accepterar du ett läggs en lagringsplats till som bara tar emot kopior av den erbjudna domänen, eftersom ett erbjudande har med sig en användare för just den domänen. Accepterar du på sidan Flotta läggs samma lagringsplats till.

### SFTP {#kind-sftp}

Adressen är `sftp://<user>@<host>:<port>/<path>`, till exempel `sftp://bv@backup.lan:22/bombvault`. Formuläret frågar efter värd, port och användare och visar BombVaults publika nyckel. Lägg till den nyckeln i användarens `~/.ssh/authorized_keys` på servern; inget annat behöver installeras där. BombVault godtar serverns värdnyckel vid första kontakten och kontrollerar den därefter.

**Hetzner Storage Box** fyller i `<user>.your-storagebox.de` och port 23. Installera nyckeln på boxen med Hetzners eget kommando, som frågar efter boxens lösenord en gång:

```sh
echo '<public key>' | ssh -p 23 <user>@<user>.your-storagebox.de install-ssh-key
```

### WebDAV: Nextcloud, ownCloud, OpenCloud {#kind-webdav}

Formuläret frågar efter serverns adress, användaren och ett applösenord. Skapa applösenordet i kontots säkerhetsinställningar, och ange användar-ID:t i stället för en e-postadress. BombVault bygger den WebDAV-sökväg som produkten använder och lämnar över anslutningen till restic via rclones miljövariabler, med lösenordet i rclones obfuskerade form. Adressen blir `rclone:bvp<id>:<path>`, där `bvp<id>` är en fjärr som bara finns i den miljön; ingenting skrivs till rclone-konfigurationen.

### Azure Blob {#kind-azure}

Adressen är `azure:<container>:/<path>`. Formuläret frågar efter lagringskontot och dess åtkomstnyckel; efter **Testa anslutning** listar det kontots containrar att välja bland, eller så skriver du in en containers namn. BombVault skickar kontot och nyckeln till restic som `AZURE_ACCOUNT_NAME` och `AZURE_ACCOUNT_KEY`.

### rclone {#kind-rclone}

Adressen är `rclone:<remote>:<path>`. Formuläret listar fjärrarna i BombVaults rclone-konfiguration att välja bland. För att ersätta den konfigurationen klistrar du in en hel `rclone.conf` under **rclone-konfiguration** och klickar på **Spara konfiguration**. Den sparas direkt och gäller för varje rclone-lagringsplats, oavsett om fönstret sedan lägger till en eller inte.

## Kopior mellan lagringsplatser med olika inloggningsuppgifter {#different-credentials}

En domän som sparas på en fjärransluten lagringsplats är källan till sina kopior. `restic copy` körs med en enda miljö, och BombVault lägger till källans inloggningsuppgifter till målets när de två inte sätter samma variabel till olika värden. En Nextcloud-lagringsplats och en B2-lagringsplats använder olika variabler, så en domän som sparas i Nextcloud kan kopieras till B2. Två S3-konton eller två rest-server-användare skulle behöva samma variabler med olika värden; restic kan inte ta båda, och chippet på kortet Domäner säger att inloggningsuppgifterna inte passar ihop.
