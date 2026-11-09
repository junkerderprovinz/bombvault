# Depanare

Un scurt FAQ. Pentru tabelul complet de depanare pe partea de gazdă pentru VM prin SSH (permission-denied, verificarea cheii gazdei, variabile de șablon lipsă și altele), vezi [ghidul de backup VM prin SSH](vm-backup-ssh-setup.md).

## Ceva nu este cablat corect {#something-is-not-wired-up-correctly}

Adaugă cardul **Integrare gazdă** în prezentare, la **Personalizează**. Verificarea integrării gazdei sondează fiecare montare și CLI (socket Docker, libvirt, restic, qemu-img, rclone) și raportează orice element lipsă, iar **Verifică acum** o rulează din nou. Începe de aici înainte să presupui o eroare: o montare lipsă sau o gazdă inaccesibilă apare imediat.

## Nu pot ajunge la interfața web {#i-cannot-reach-the-web-ui}

BombVault servește HTTPS din start pe portul `3443` (certificat auto-semnat), deci deschide `https://<your-unraid-ip>:3443`. Acceptă avertismentul de certificat auto-semnat sau pune BombVault în spatele unui reverse proxy cu propriul tău certificat. Dacă rulezi cu `HTTP_ONLY=true`, servește în schimb HTTP simplu pe portul `3000` (destinat utilizării în spatele unui proxy care termină TLS).

## Mi-am pierdut APP_KEY {#i-lost-my-app_key}

`APP_KEY` derivă parola depozitului restic. Fără el, fără kitul de recuperare și fără o depunere de cheie în grupul tău, backupurile criptate nu pot fi recuperate. De aceea prezentarea arată **Salvează-ți kitul de recuperare** până îl descarci. Vezi [Off-site și recuperare](offsite-recovery.md#recovery-kit). Generează o cheie cu `openssl rand -hex 32` și păstreaz-o în afara serverului înainte să te bazezi pe orice backup.

## Am uitat parola de conectare {#i-forgot-the-login-password}

Alege **Ai uitat parola?** pe pagina de autentificare. BombVault scrie un cod de 12 caractere în jurnalul containerului său (în Unraid: fila Docker, pictograma BombVault, **Logs**). Tastează-l cu o parolă nouă în 10 minute. Resetarea dezactivează și autentificarea în doi pași, elimină toate cheile de acces, revocă toate tokenurile API, cheile MCP și autentificările OAuth, încheie toate celelalte sesiuni și oprește împerecherea telefoanelor. Adaugă din nou cheile de acces la **Setări, Securitate**, pornește tot acolo împerecherea telefoanelor și creează tokenuri și chei noi unde mai ai nevoie de ele. Vezi [Model de securitate](configuration.md#login).

## Backupurile s-au oprit și spun că cheia nu mai poate fi deschisă {#backups-stopped-and-say-the-key-can-no-longer-be-opened}

Acest server a preluat cheia de backup a altui server în Asistentul de urgență, iar `APP_KEY` a fost schimbat după aceea. Cheia preluată este sigilată cu vechiul APP_KEY, așa că BombVault nu o poate deschide și nu face niciun backup, în loc să scrie cu cheia greșită. **Necesită atenție** din prezentare o numește. Pune APP_KEY la loc, cum era, sau încarcă din nou kitul de recuperare pe pagina Restaurare, la **Urgență**.

## Un telefon ajunge pe pagina de autentificare {#a-phone-lands-on-the-login-page}

Un telefon împerecheat primește sesiunea prin grup doar cât timp **Telefoanele se pot împerechea cu cele douăsprezece cuvinte ale grupului** este pornit la **Setări, Securitate**. O resetare a parolei îl oprește. O aplicație mai veche decât serverul nu primește nici ea sesiune până se actualizează. Un server adăugat după adresă cere mereu parola. Un telefon eliminat de pe pagina Instanțe revine dezactivat când i se deschide din nou aplicația. Pornește **Interoghează această instanță** la **Monitorizare** în detaliile lui ca să-l lași să intre.

## Backupul VM nu se conectează {#vm-backup-will-not-connect}

Backupul VM comunică cu libvirt prin SSH, niciodată printr-o montare.

- Confirmă că SSH este activat pe gazdă și că cheia publică a BombVault este autorizată în `/root/.ssh/authorized_keys` (Setări, Integrări, SSH al gazdei arată cheia și un buton **Testează conexiunea**).
- Pe o rețea `br0.x` personalizată, setează `LIBVIRT_HOST` la IP-ul LAN al Unraid (containerul nu poate ajunge acolo la gazdă prin `host.docker.internal`). Activează **Settings, Docker, Host access to custom networks**.
- Dacă ai schimbat portul SSH al Unraid, setează `LIBVIRT_SSH_PORT` să corespundă.
- Pe TrueNAS, setează `LIBVIRT_URI` cu socket-ul libvirt propriu al TrueNAS, dă aplicației capabilitatea `SETPCAP` și adaugă cheia la root, la **Credentials, Users** (vezi [TrueNAS SCALE](getting-started.md#truenas)).
- Diagnosticul complet pas cu pas (test de accesibilitate, rutare VLAN, `Permission denied (publickey)`, `Host key verification failed`) este în [ghidul de backup VM prin SSH](vm-backup-ssh-setup.md).

## Un instantaneu VM live nu a rulat {#a-live-vm-snapshot-did-not-run}

Instantaneele live au nevoie de discul pe `/mnt/cache` (sau `/mnt/diskX`), nu `/mnt/user`. Cu qemu guest agent în VM instantaneul este liniștit, iar fără el instantaneul este consistent la nivel de crash. Pe o VM oprită, live revine automat la controlat. Un backup controlat oprește VM-ul, face backup discurilor, apoi îl repornește, așa că este mereu consistent. Așteaptă până la zece minute ca oaspetele să se oprească și nu îl oprește niciodată forțat, așa că un oaspete Windows care instalează actualizări nu este întrerupt la jumătate. Un oaspete care încă rulează după aceea este lăsat în pace, iar acel backup eșuează cu motivul.

## Un backup a eșuat cu „repository is already locked” {#a-backup-failed-with-repository-is-already-locked}

De obicei este un lock restic orfan rămas când containerul a fost actualizat sau repornit în mijlocul unei operațiuni. BombVault detectează un lock dovedit orfan, îl curăță și reîncearcă o dată, automat. După o repornire așteaptă până când un astfel de lock a stat zece minute fără reîmprospătare. Un restic care încă rulează își reîmprospătează lockul la fiecare cinci minute, iar BombVault nu elimină niciodată un lock care încă este reîmprospătat, pentru că poate aparține unui al doilea BombVault care scrie în același depozit. Un lock care a stat 30 de minute fără reîmprospătare este eliminat, indiferent cine l-a pus. Dacă eroarea persistă, deschide cardul **Verifică copiile de rezervă** de la **Setări, Verificare** și alege **Deblochează** la **Mai mult**, în rândul secțiunii afectate, ca să ștergi manual un lock vechi. Nici **Deblochează** nu atinge un lock care încă este reîmprospătat și îți spune când a fost reîmprospătat ultima dată. O problemă reală tot iese la iveală, nu este ascunsă.

## Copia mea off-site nu s-a întâmplat după un backup {#my-off-site-copy-did-not-happen-after-a-backup}

Replicarea off-site este best-effort prin concepție, așa că un sughiț off-site nu face niciodată să eșueze backupul local. Verifică când copiază destinația: cardul ei **Copiază** spune **După fiecare copie de rezervă** sau numește un plan propriu, iar **Setări, Programare, Copii externe pe secțiuni** poate da unei secțiuni un plan propriu. Folosește **Replică acum** pe rândul secțiunii din cardul **Copii externe** de la **Setări, Destinații** pentru o rulare la cerere și urmărește antetul de stare din prezentare, care spune **Copierea off-site rulează** cât timp rulează. O copiere eșuată apare la **Necesită atenție**.

## Certificatul unui rest-server nu este de încredere {#a-rest-servers-certificate-is-not-trusted}

restic refuză un certificat pe care nu îl cunoaște, iar eroarea spune unde să-l pui. Copiază certificatul auto-semnat al rest-server-ului, sau pe cel al autorității care l-a semnat, ca fișier PEM în folderul `certs` din folderul de date al BombVault (`/config/certs` în container, `appdata/bombvault/certs` pe Unraid) și testează din nou. Nu este nevoie de repornire.

## Un backup spune că datele din afara Host Data nu au fost salvate {#data-outside-host-data}

Containerul ține date într-un folder pe care BombVault nu îl poate citi prin Host Data. Backupul salvează definiția și fiecare folder la care ajunge, își colorează linia din jurnalul de activitate în portocaliu și numește folderele lăsate pe dinafară. Două cazuri sunt frecvente: un ixVolume pe TrueNAS sub `/mnt/.ix-apps` și un volum Docker cu nume sub `/var/lib/docker/volumes`, de exemplu volumul bazei de date dintr-un stack compose. Cardul **Foldere de salvat** din fila **Setări** a containerului arată fiecare astfel de folder cu calea lui din container și calea de pe gazdă, marcat cu roșu.

Un volum Docker anonim nu este raportat. Docker îl creează singur pentru un folder declarat de imagine și pe care nu l-a mapat nimeni, îi dă un nume de 64 de cifre hexazecimale și îl ține sub `/var/lib/docker/volumes`. De obicei conține un cache, așa că backupul rămâne verde, iar cardul doar îl afișează, cu o notă că este anonim.

- **Vrei să salvezi aceste date.** Mapează folderul pe o cale a gazdei sub Host Data, de exemplu un folder din share-ul appdata, iar următorul backup îl include.
- **Nu ai nevoie de ele.** Adaugă una dintre cele două căi ale lui ca linie în cardul **Modele de excludere** din aceeași filă: calea din interiorul containerului, de exemplu `/cache`, sau calea de pe gazdă pe care o numește avertismentul, de exemplu `/var/lib/docker/volumes/jellyfin_cache/_data`. De la următorul backup, avertismentul lasă folderul deoparte, iar previzualizarea de sub câmp spune de ce. Merge și o linie pentru un folder de deasupra, un nume simplu sau `/` nu.

## Un backup arată Amânat {#deferred-backups}

Aplicația containerului era ocupată, așa că backupul lui așteaptă și este încercat din nou de la sine. Linia din jurnalul de activitate spune de ce și când are loc următoarea încercare. Cardul **Când aplicația e ocupată** din fila **Setări** a containerului spune același lucru și când se termină așteptarea. Nu trebuie să faci nimic. Ca să faci backupul imediat, folosește **Copiază acum**: un backup pornit de tine nu așteaptă niciodată, nici la un cod de ieșire 75.

- **Propria ta verificare.** Fă ca **Comandă pre-backup** să iasă cu codul 75 cât timp aplicația e ocupată, de exemplu `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Orice alt cod diferit de 0 face backupul să eșueze ca înainte, așa că un script stricat apare tot la **Necesită atenție**.
- **Plex, Jellyfin sau Emby nu este întrebat niciodată.** BombVault întreabă întâi serverul la adresa proprie a containerului și, când acolo nu răspunde nimic, rulează curl sau wget în interiorul containerului. O imagine fără niciunul dintre ele nu poate fi întrebată prin ea însăși. Jurnalul spune atunci asta. Cu **Fă copia când serverul nu răspunde** pornit, un astfel de backup rulează oricum, cu el oprit așteaptă.
- **Jellyfin sau Emby refuză.** Verifică **Cheie API**. Se creează în dashboardul serverului, la API keys.
- **Plex refuză.** BombVault citește tokenul din `Preferences.xml` din folderul montat la `/config`. Un server la care nu s-a conectat niciodată nimeni nu are token și răspunde doar acolo unde permite accesul fără el.
- **Un film pus pe pauză ține backupul în așteptare toată noaptea.** Setează **Ignoră sesiunile în pauză după (min)**, de exemplu la 30.
- **Verificarea traficului nu lasă niciodată backupul să treacă.** Ridică **Ocupat de la (KiB/s)** peste cât trimite containerul când nu îl folosește nimeni. Imediat după pornirea BombVault, verificarea așteaptă până a urmărit containerul atâtea minute câte spune **Liniștit timp de (min)**.
- **Backupul nu rulează niciodată noaptea.** Cu un interval setat, încercările au loc doar în interiorul lui. O așteptare care își atinge termenul în afara intervalului se încheie totuși la termen.

## Un backup mare a fost oprit din lipsă de memorie {#a-large-backup-was-killed-for-running-out-of-memory}

Setează **Limită de memorie per copie (MiB, 0 = fără)** în cardul **Performanță** de la **Setări, Sistem**, cu mult sub cât are voie să folosească containerul. Aproape de limită, restic eliberează memorie mai des, ceea ce costă puțină viteză, așa că backupul încetinește în loc să fie oprit.

## O restaurare s-a oprit înainte de a începe {#a-restore-aborted-before-it-started}

Înainte ca ceva să fie oprit sau eliminat, restaurarea rulează o verificare de conflicte prealabilă: verifică dacă IP-ul static al containerului și porturile publicate ale gazdei sunt libere. Dacă un alt container deține deja unul, se oprește cu un mesaj clar, acționabil, în loc să lase o restaurare pe jumătate terminată. Eliberează portul sau IP-ul în conflict, apoi reîncearcă.

Dacă rețeaua containerului nu există, de exemplu pentru că aplicația lui TrueNAS a fost ștearsă, verificarea înainte de restaurare numește rețeaua, iar restaurarea se oprește înainte să schimbe ceva. Reinstalează aplicația în TrueNAS cu același nume, ceea ce creează din nou rețeaua, apoi restaurează-i datele.

## Un export în clar a eșuat în loc să scrie un fișier {#a-plain-export-failed-instead-of-writing-a-file}

Dacă **Criptează exporturile cu age** este pornit la **Setări, Sistem**, dar nu este setat niciun destinatar valid, un export eșuează cu o eroare clară în loc să scrie text simplu. Adaugă un destinatar valid (o cheie publică age sau o cheie publică SSH) sau oprește criptarea dacă vrei ca exportul să fie text simplu. Vezi [Exporturi în clar](backups.md#exports).

## Un dump de bază de date a eșuat {#a-database-dump-failed}

Un dump eșuat nu duce niciodată la eșec backupul din jurul lui. Este consemnat ca o rulare eșuată de sine stătătoare, iar motivul spune ce trebuie reparat.

- **Autentificare refuzată.** Dumpul se autentifică cu variabilele de parolă ale containerului însuși (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` sau versiunile lor `_FILE`). Verifică-le pe containerul bazei de date. O variabilă `_FILE` care arată spre un secret pe care utilizatorul containerului nu îl poate citi eșuează la fel.
- **Privilegii lipsă.** Cu o parolă de root aleatoare, dumpul se poate autentifica doar ca utilizator al aplicației, deci conține doar acea bază, iar MySQL 8.4 și mai nou îl poate refuza cu totul. Dă containerului o parolă de root adevărată sau oprește-i dumpul.
- **Tabelele de sistem au nevoie de o actualizare.** MariaDB refuză dumpul când tabelele sale de sistem vin dintr-o versiune mai veche (eroarea 1558). Adaugă variabila `MARIADB_AUTO_UPGRADE=1` și repornește containerul, sau rulează o dată `mariadb-upgrade` înăuntru.
- **Niciun instrument de dump.** O imagine subțiată sau făcută în casă, fără `pg_dump`, `mysqldump` sau `mariadb-dump`, nu poate fi dumpată. Folosește imaginea oficială sau oprește dumpul.
- **O limită de timp.** Un dump primește `DB_DUMP_MAX_HOURS` (6 implicit), backupul din jur primește `BACKUP_MAX_HOURS`, iar un dump care nu mai avansează este tăiat după `BACKUP_STALL_HOURS`. În spatele ultimului caz stă de obicei un lock ținut de aplicație. Ridică limita care s-a declanșat sau fă dumpul cât aplicația e liniștită.
- **Containerul este în pauză sau repornește.** Dumpul vorbește cu serverul în funcțiune. Dacă containerul repornește la nesfârșit, jurnalul lui spune de ce.
- **Un dump deteriorat nu a putut fi înlăturat.** Un dump pe care BombVault nu l-a putut termina este șters. Când ștergerea aceea eșuează, dumpul rămâne în listă marcat ca deteriorat și îl poți șterge de acolo.

## Un import a eșuat {#an-import-failed}

Un import oprește containerul, pune deoparte folderul lui de date și lasă imaginea să creeze unul gol în loc. Dacă eșuează un pas de dinaintea importului propriu-zis, folderul vechi este pus la loc automat. Dacă eșuează importul, containerul rămâne cu folderul proaspăt, iar cel vechi stă alături ca `<folder de date>.bombvault-before-import-<marcaj de timp>`. Mesajul de eroare al rulării numește calea exactă.

Un folder de date care este un set de date propriu, ca pe TrueNAS, nu poate fi redenumit. Importul lasă atunci folderul pe loc și îi mută conținutul în folderul păstrat de lângă el, așa că setul de date de deasupra are nevoie de loc pentru el. Importul verifică asta înainte să oprească containerul.

Ca să îl pui la loc manual: oprește containerul, redenumește folderul de date curent ca să îl dai la o parte, redenumește folderul păstrat înapoi la numele original și pornește containerul. La un set de date, mută conținutul în loc să redenumești folderele. Pe Unraid, managerul de fișiere din fila Shares face asta.

## Un backup al unui set de date ZFS a eșuat sau a sărit un set {#zfs-datasets}

Fiecare problemă are un cod de motiv între paranteze, iar pagina [Seturi de date ZFS](zfs-datasets.md#reason-codes) le enumeră pe toate cu remedierea. Cele mai frecvente trei:

- **`snapshot-loop`**: instantaneul nu a ajuns la BombVault, pentru că Host Data nu transmite montările noi. Editează containerul, setează Access Mode pentru Host Data la Read/Write - Slave și repornește BombVault.
- **`key-not-loaded`**: un set de date criptat a cărui cheie nu este încărcată este sărit. Încarcă cheia cu `zfs load-key` și montează setul. Următorul backup îl include.
- **`ssh-auth`**: serverul a refuzat cheia BombVault. **Conexiune ZFS la acest server**, deschisă cu **Vezi conexiunea** pe pagina Copii de rezervă, arată comanda care o autorizează. Rulează-o o dată pe server.

## Un element rămâne la „Învață N/10” {#an-entry-stays-at-learning-n10}

Majoritatea verificărilor de anomalii pornesc după 10 backupuri reușite ale unui element, iar numărătoarea reîncepe după **Marchează ca așteptată** și după ce selecția elementului s-a schimbat. Un element care nu este programat nu învață, iar un container fără appdata nu are de unde învăța, lucru pe care îl spune insigna lui. **Progresul învățării** de pe pagina Anomalii arată un inel din zece părți pe element.

## Retenția a încetat să șteargă backupurile vechi ale unui element {#retention-stopped-deleting-old-backups-of-one-entry}

O anomalie critică deschisă le ține: sursa elementului este aproape goală, s-a micșorat brusc sau un backup a stocat din nou cea mai mare parte din datele ei. Deschide anomalia din insigna elementului. Dacă lipsesc date sau au fost criptate, restaurează mai întâi din ultimul backup bun la care trimite. Apoi confirmă anomalia sau marcheaz-o ca așteptată dacă schimbarea a fost a ta, iar rularea următoare curăță ca de obicei. Previzualizarea retenției marchează un astfel de element ca păstrat. Pentru un element ZFS doar setul de date numit în anomalie își păstrează backupurile vechi, iar celelalte seturi de date din arbore sunt curățate ca de obicei.

## Curățarea manuală spune că unele elemente au fost păstrate {#manual-prune-says-some-entries-were-kept}

Aceeași cauză: curățarea (prune) lasă în pace backupurile vechi ale unui element cu o asemenea anomalie și numește elementul în mesajul ei. Tot restul este curățat ca de obicei.

## Un backup eșuează cu „on no mounted pool or share” {#no-mounted-pool}

Pe Unraid, `/mnt` trăiește în memorie, așa că un folder de acolo peste care nu este montat niciun pool dispare la următoarea repornire și rămâne ascuns după ce poolul revine. BombVault refuză să scrie acolo un backup sau un ZIP flash, chiar și atunci când în acel folder se află deja un depozit de la o scriere anterioară. Montează poolul și rulează din nou backupul.

## Importul istoricului spune că un depozit nu a putut fi citit {#history-import-says-a-repository-could-not-be-read}

După actualizare, BombVault citește o dată dimensiunile backupurilor anterioare din fiecare depozit. Un depozit pe care nu l-a putut atinge atunci, de exemplu o destinație off-site căzută sau un share nemontat, este numărat în cardul **Anomalii** de la **Setări, Verificare** și reîncercat o dată pe zi. Elementele lui învață între timp din backupurile noi.

## Avertismentul despre spațiul pe disc nu se potrivește cu panoul Unraid {#the-disk-space-warning-does-not-match-the-unraid-dashboard}

Pe share-ul de utilizator Unraid (`/mnt/user`) spațiul liber este cel al întregului array, nu al unui singur disc. Destinațiile la distanță pe care BombVault le atinge prin rclone, de exemplu SFTP, Hetzner Storage Box, SMB, WebDAV sau un drive cloud, sunt măsurate când partea îndepărtată își raportează spațiul liber, la fel ca o adresă `sftp:` scrisă de mână. Depozitele S3, B2 și REST nu au o valoare: destinația lor spune **Dimensiune necunoscută**, iar cardul **Anomalii** le listează ca nemăsurate.

## Un asistent IA nu se poate conecta {#an-ai-assistant-cannot-connect}

Pagina [Server MCP](mcp.md#troubleshooting) explică ce înseamnă fiecare cod de stare și fiecare refuz al punctului de conectare MCP și ce poți face.

## Containerul se tot repornește sau pare nesănătos {#the-container-keeps-restarting-or-looks-unhealthy}

BombVault raportează sănătos sau nesănătos din propriul `/api/health`. Un instrument de auto-vindecare (precum Autoheal) îl poate reporni automat dacă motorul se blochează vreodată. Verifică jurnalul containerului și cardul **Integrare gazdă** pentru cauza de fond.

## O destinație refuză să se salveze {#location-refused}

BombVault verifică o destinație înainte s-o salveze, iar refuzul spune de ce într-o propoziție. Cele obișnuite:

- **„This location lies inside another repository or target, or contains one.”** Un depozit în interiorul unui depozit le-ar strica pe amândouă. Mesajul numește celălalt loc. Alege alt folder sau elimină-l mai întâi pe acela.
- **Este un depozit pe care l-ai configurat.** Destinația este unul dintre rândurile de sub **Depozite** sau se află în interiorul ori în jurul unuia. Alege alt loc sau elimină mai întâi acel depozit.
- **Același loc cu dosarul propriu al unei secțiuni.** O copie pe dosarul propriu al unei secțiuni și-ar copia backupurile peste ele însele la fiecare rulare, așa că este refuzată. Alege alt loc sau vezi [Folderul propriu al unei secțiuni](offsite-recovery.md#own-folder).
- **„That is not an off-site target of this domain”** venea de la o legătură către o destinație eliminată. O versiune actuală le curăță la pornire și la import. Dacă tot îl vezi, actualizează BombVault.
- **O cale completă a gazdei**, de exemplu `/mnt/user/backups`. BombVault rulează într-un container, așa că vrea calea de sub Host Data, `user/backups`, iar mesajul o numește pe cea de introdus.

## O destinație la distanță spune că depozitul ei a dispărut {#repository-gone}

O destinație care avea backupuri și brusc nu mai are niciun depozit nu primește de la sine unul nou, gol, pentru că acela l-ar ascunde pe cel real odată ce un disc revine. Un rest-server al cărui disc nu s-a montat este cauza obișnuită, deci montează discul din nou și rulează încă o dată backupul sau copierea. Doar dacă ai golit sau ai șters intenționat vechiul depozit, apasă **Creează aici un depozit nou** pe pagina destinației. Pornește de la zero pentru toate secțiunile acelei destinații deodată.

## O autentificare cloud a expirat {#cloud-sign-in-expired}

Testul sau următoarea copiere spune atunci că autentificarea a expirat. **Schimbă conexiunea** pe pagina destinației te autentifică din nou.

- **Box, Jottacloud și altele cu tokenuri de reîmprospătare de unică folosință:** BombVault păstrează tokenul pe care rclone l-a reîmprospătat, inclusiv peste reporniri și în kitul de recuperare. După o restaurare dintr-un backup mai vechi al setărilor, tokenul din el poate fi expirat, așa că autentifică-te încă o dată. Depozitul rămâne cum este.
- **iCloud Drive:** Apple încheie autentificarea după 30 de zile. Are nevoie de un nou token de încredere și de cookie-uri din `rclone config`.
- **OneDrive:** o autentificare cu mai multe drive-uri întreabă pe care să-l folosească, așa că o destinație rămasă fără alegere rămâne inutilizabilă până alegi. Pentru OneDrive Business și Google Drive vezi nota despre datele curățate la [Destinații](offsite-recovery.md#destinations).

## „The off-site copy of the last backup is still running” {#copy-still-running}

Backupul dinainte s-a terminat, iar copia lui încă este pe drum, ceea ce blochează un backup nou al acelei secțiuni. Așteaptă până când antetul de stare nu mai spune **Copierea off-site rulează**. Butoanele care pornesc un backup spun același lucru cât așteaptă. Prima copie către o destinație nouă poate dura ore, vezi [exemplul S3](offsite-recovery.md#example-s3).

## O replică ZFS se oprește pentru că ținta are instantanee mai noi {#replica-target-newer}

Ținta are instantanee mai noi decât cel mai nou pe care îl împarte cu acest server, de exemplu pentru că setul de date de aici a fost dat înapoi sau copia de pe țintă a fost schimbată. Cardul Replică și istoricul rulărilor numesc instantaneul de la **Roll the target back to**. Rulează `zfs rollback -r` la acel instantaneu pe țintă, pentru fiecare set de date pe care îl numește rularea, și apasă **Replicează acum**. Vezi [Cât rămân instantaneele](zfs-datasets.md#replica-retention).

## O VM cu instantanee Unraid eșuează și numește un fișier {#vm-chain}

BombVault urmărește lanțul de bază al fiecărui disc și face backup fiecărui fișier din el. Un lanț care nu poate fi urmărit până la capăt face backupul să eșueze, în loc să lase deoparte o parte din disc, iar mesajul numește fișierul. Verifică dacă fișierul există sub `/mnt` și dacă instantaneul este încă listat în VM Manager. Un fișier din afara Host Data nu poate fi citit deloc. Vezi [VM-uri cu instantanee Unraid](vm-backup-ssh-setup.md#unraid-snapshots).

## Copiile off-site așteaptă decizia ta după o reconstruire {#copies-paused}

După **Descoperă copii de rezervă** sau după Asistentul de urgență fără setări restaurate, sau când o secțiune găsește backupuri proprii de dinaintea acestei baze de date, copiile off-site ale acelei secțiuni așteaptă, pentru că BombVault nu mai știe ce elemente ai lăsat deoparte. **Necesită atenție** și pagina destinației te întreabă despre asta. Apasă **Preia-le ca fiind ale acestui server** ca să continui, sau **Verifică mai întâi regulile** ca să te uiți la regulile secțiunii înainte. Importul unui fișier de setări nu ia decizia în locul tău. Vezi [După o reconstruire](offsite-recovery.md#after-a-rebuild).

## Notificările nu ajung {#notifications-do-not-arrive}

Începe cu **Trimite test** pe canal, la **Setări, Notificări**. Rândul de sub canal spune ce s-a întâmplat.

- **Nu se trimite nimic.** Verifică dacă acel canal este pornit și ce are la **Notifică**. Cu **Niciodată** nu trimite nimic, iar cu **Doar la eșec** o rulare reușită rămâne tăcută. Cu **Rezumă rulările programate** pornit, o rulare programată trimite un singur rezumat la sfârșit.
- **O adresă nouă.** După ce schimbi unde trimite un canal, de exemplu serverul Matrix, Gotify sau ntfy, ori gazda unui canal de e-mail, câmpul cere din nou tokenul sau parola, iar noua adresă se salvează abia după ce le scrii. BombVault nu trimite vechiul secret către un loc nou.
- **Gotify** spune că serverul a refuzat tokenul aplicației: copiază-l din nou din aplicația din Gotify. **URL-ul serverului** este adresa de bază, fără `/message`.
- **Discord, Slack și ntfy** sunt canale separate. O adresă Discord într-un canal **Webhook** primește JSON pe care Discord nu îl înțelege, așa că adaugă pentru ea un canal **Discord**.
- **Webhook:** un substituent pe care BombVault nu îl cunoaște rămâne cum l-ai scris, așa că verifică ortografia, de exemplu `%%title%%`. Pentru un conținut JSON, lasă antetul `Content-Type` pe `application/json`, ca valorile să fie codificate ca JSON.
- **Notificările Unraid** merg prin conexiunea SSH a gazdei, așa că testeaz-o mai întâi la **Setări, Integrări**. Pe altă platformă rămân oprite, iar canalul spune asta.
- **Un e-mail de notificare nu vine niciodată.** Verifică **Criptare** față de port, de obicei **STARTTLS** pe 587 sau **TLS (implicit)** pe 465.

Pingurile Healthchecks nu depind de **Notifică**, vezi [Healthchecks](notifications.md#healthchecks).

## Aplicația Android nu poate deschide un server {#android-cannot-open}

Ecranul de start spune de ce, pe cardul serverului:

- **Neconectat:** nici adresa serverului, nici relay-ul nu răspund. La **Fără relay**, aplicația ajunge la un server doar la adresa lui proprie.
- **Verifică certificatul:** serverul arată alt certificat decât cel în care ai avut încredere. Aplicația îl deschide abia după ce ai încredere în cel nou, așa că asigură-te că schimbarea este a ta.
- **Este nevoie de autentificare:** un server adăugat după adresă cere parola. Vezi și [Un telefon ajunge pe pagina de autentificare](#a-phone-lands-on-the-login-page).
- **Cheia s-a schimbat:** ecranul de start arată amprenta cheii noi, iar pagina se deschide abia după ce ai încredere în ea. Vezi [În afara casei](android.md#away).

## Alt utilizator nu poate citi depozitul local {#repository-permissions}

Depozitul local rămâne citibil pentru alți utilizatori, de exemplu un utilizator de sincronizare prin SMB. Înainte de 10.4.1, fișierele scrise de un prune, o etichetare, un pull, o mutare de secțiune, o ștergere sau un backup de stack puteau rămâne necitibile până în noaptea următoare. Actualizează BombVault și devin citibile imediat. Depozitul auto-backupului rămâne citibil doar de root, intenționat, pentru că ține setările BombVault.

## Tot blocat? {#still-stuck}

- Citește paginile complete [Configurare](configuration.md) și [Off-site și recuperare](offsite-recovery.md).
- **Raport de eroare** din cardul **Ajutor** de la **Setări, Info** descarcă un ZIP pentru un issue GitHub: verificarea gazdei, configurația ta fără niciun secret, rulările recente, ce urmează în programare și jurnalul recent de când a pornit containerul ultima dată. Conține și ultimul dump al fiecărei baze de date, elementele ZFS cu montările lor, anomaliile deschise și câte chei MCP există. Parolele, tokenurile și configurația rclone sunt eliminate, iar pachetul spune asta în propriul manifest. Cere o parolă de conectare. Pentru o cădere care a repornit containerul, uită-te la `docker logs`.
- Întreabă pe [firul de suport Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Deschide o [problemă pe GitHub](https://github.com/junkerderprovinz/bombvault/issues).
