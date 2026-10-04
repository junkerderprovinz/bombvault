# Configurare

Această pagină acoperă variabilele de mediu ale containerului, montările pe care le oferă șablonul, backupul VM prin SSH și configurarea off-site. **Căile depozitelor** de backup sunt configurate în interiorul aplicației (Setări, Stocare, Căi de backup), nu prin variabile de mediu.

## Variabile de mediu

| Variabilă | Obligatorie | Descriere |
|---|---|---|
| `APP_KEY` | **Da** | Secret hex de 32 de octeți (64 de caractere hex) folosit pentru a deriva parola depozitului restic. Generează cu `openssl rand -hex 32`. Păstrează-l în siguranță: pierderea lui face ca backupurile criptate să nu mai poată fi recuperate. |
| `LIBVIRT_HOST` | Pentru VM-uri și seturi de date ZFS | Gazda Unraid accesată prin SSH pentru backupul VM (implicit `host.docker.internal`; șablonul precompletează un placeholder cu IP-LAN). Folosește IP-ul LAN al Unraid, obligatoriu pe o rețea `br0.x` personalizată. Folosit și pentru backup-urile seturilor de date ZFS (câmpul de șablon **Host SSH: Address**); substituentul `192.168.x.x` contează ca nesetat. |
| `LIBVIRT_SSH_PORT` | Nu | Portul SSH al gazdei pentru backupul VM (implicit `22`). Câmpul de șablon **Host SSH: Port**, și pentru seturile de date ZFS. |
| `LIBVIRT_SSH_USER` | Nu | Utilizatorul SSH de pe gazdă pentru backupul VM (implicit `root`). Câmpul de șablon **Host SSH: User**, și pentru seturile de date ZFS. |
| `LIBVIRT_URI` | Nu | URI-ul complet de conexiune libvirt, folosit **ca atare** în locul construirii unuia dintre cele trei variabile `LIBVIRT_*` de mai sus (care sunt apoi ignorate pentru șirul de conexiune). Nesetat implicit. Necesar pe TrueNAS Scale, al cărui libvirtd ascultă pe un socket non-standard pe care forma construită nu îl poate exprima: `qemu+ssh://<user>@<truenas-host>/system?socket=/run/truenas_libvirt/libvirt-sock`. Vezi secțiunea TrueNAS Scale din [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md). Dacă este un URI `qemu+ssh://`, fiecare dintre `LIBVIRT_HOST`, `LIBVIRT_SSH_USER` și `LIBVIRT_SSH_PORT` care nu este setată se ia din el, și pentru comenzile SSH proprii ale BombVault (transferul NVRAM, seturile de date ZFS). |
| `PORT` | Nu | Portul HTTP (implicit `3000`; folosit doar cu `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Nu | Portul HTTPS (implicit `3443`; șablonul îl publică 1:1, deci WebUI răspunde la `https://<ip>:3443`). |
| `HTTP_ONLY` | Nu | Setează `true` pentru a dezactiva ascultătorul HTTPS auto-semnat și a servi doar HTTP simplu (pentru utilizare în spatele unui reverse proxy care termină TLS). |
| `BIND_HOST` | Nu | Adresa pe care ascultă WebUI (implicit `0.0.0.0`, toate interfețele). Las-o nesetată în container, ale cărui porturi publicate au nevoie de toate interfețele; `127.0.0.1` se potrivește unei rulări în afara Docker. Healthcheck-ul întreabă aceeași adresă. |
| `TRUSTED_PROXY` | Nu | Adrese sau intervale CIDR, separate prin virgulă, ale proxy-ului invers din fața BombVault (de exemplu `192.168.20.11` sau `10.0.0.0/8`). Doar de la aceste noduri se crede antetul `X-Forwarded-For`, iar limitarea autentificărilor numără atunci eșecurile pe client real, în loc să pună toți clienții din spatele proxy-ului în aceeași găleată. Nesetat (implicit) înseamnă că nimeni nu este crezut: un antet crezut necondiționat ar lăsa pe oricine să își aleagă propria găleată. |
| `HOST_SOURCE_ROOT` | Nu | Calea gazdei montată ca **Host Data** (implicit `/mnt`). BombVault traduce sursele de bind-mount raportate de Docker în căi sub această montare. Schimbă doar dacă ai montat o altă rădăcină de gazdă. |
| `DATA_ROOT_SEGMENTS` | Nu | Nume de segmente de cale separate prin virgulă care marchează o sursă de bind-mount ca date de backup (implicit `appdata`, conform convenției `/mnt/user/appdata/<container>` a Unraid). Bind-mount-ul unui container este selectat automat pentru backup atunci când ORICARE segment din listă apare ca segment complet de cale în sursa sa de pe gazdă; de exemplu, `DATA_ROOT_SEGMENTS=appdata,config` preia și un bind `.../config`. Vezi [Detectarea surselor de backup](#backup-source-detection) pentru celelalte moduri, mereu active, prin care este găsit folderul de date al unui container. |
| `PLATFORM` | Nu | Forțează platforma pe care BombVault se consideră că rulează, în loc să o detecteze automat: `unraid`, `generic` sau `truenas` (nesetat implicit: detectează automat Unraid sondând markerul său `dockerMan` sub montarea flash-ului, altfel `generic`; o valoare nerecunoscută revine de asemenea la `generic`, înregistrată în jurnal). Setează-o explicit pe o gazdă Docker generică sau pe TrueNAS Scale, în loc să te bazezi pe auto-sondarea specifică Unraid; fișierul compose generic face exact acest lucru. Schimbă convenția de rezervă pentru appdata, valorile implicite ale destinației de restaurare între instanțe și dacă pașii specifici Unraid de notificare/plugin însoțitor sunt încercați deloc (vezi `internal/platform`). |
| `BOMBVAULT_SELF_CONTAINER` | Nu | Numele containerului BombVault însuși, astfel încât să nu-și facă niciodată backup (și deci să nu se oprească) singur. |
| `BACKUP_MAX_HOURS` | Nu | Numărul maxim de ore de ceas pe care o singură rulare de backup îl poate ține blocajul de domeniu înainte de a fi forțat anulată (o gardă astfel încât o rulare blocată să nu poată bloca domeniul la nesfârșit). Gol (implicitul) folosește `48`. Ridică-l pentru backupuri cloud foarte mari sau lente (o rulare anulată la limită eșuează cu `context deadline exceeded`). Setează `0` pentru a dezactiva complet limita. |
| `BACKUP_STALL_HOURS` | Nu | Orele în care un backup poate să nu facă **niciun progres** înainte de a fi anulat. Gol (implicit) folosește `2`; setează `0` pentru a nu anula niciodată la blocare. Este cea mai fină dintre cele două gărzi și de obicei cea care se declanșează: urmărește dacă se mai întâmplă ceva, nu cât durează rularea, așa că un backup de mai mulți terabytes lent dar sănătos este lăsat în pace, în timp ce unul blocat pe o partajare care nu răspunde este oprit în câteva ore în loc de zile. După 30 de minute de liniște se scrie un avertisment în jurnal, înainte de a anula ceva. Scanarea contează ca progres: restic nu scrie niciun octet cât timp parcurge un arbore mare, iar acea fază este urmărită prin totalurile sale de fișiere și octeți, nu prin octeții scriși. Cele două variabile sunt independente, iar `BACKUP_MAX_HOURS` limitează în continuare fazele de după backupul propriu-zis (retenție, statistici, copie off-site), unde nu există contoare de urmărit. |
| `DB_DUMP_MAX_HOURS` | Nu | Orele în care un dump automat de bază de date poate rula înainte să fie oprit. Gol (implicit) înseamnă `6`; sunt permise valori de la `1` la `48`, iar limita rămâne cu o oră sub `BACKUP_MAX_HOURS` (la jumătatea acesteia când e sub două ore), astfel încât un dump lung să fie tăiat de propria limită și raportat ca atare, în loc să tragă backupul după el. Un dump care nu mai avansează este oprit mai devreme, după `BACKUP_STALL_HOURS`. Un dump oprit eșuează de unul singur, iar backupul containerului continuă. Pe Unraid adaugi variabila la containerul BombVault cu **Add another Path, Port, Variable**. |
| `TZ` | Nu | Fusul orar pentru programator (de exemplu `Europe/Berlin`). **Dacă nu este setată, toate programările rulează în UTC**: o programare la 02:30 pornește atunci la 02:30 UTC, nu la ora locală. Pe Unraid nu setați niciodată acest lucru: sistemul transmite propriul fus orar fiecărui container. |

## Montări

Montează socket-ul Docker, flash-ul (`/boot`) și rădăcina **Host Data** (`/mnt`) așa cum se arată în șablonul CA. Atât *sursele* cât și *destinațiile* backupurilor se află sub Host Data, iar aceasta este montată **slave** astfel încât o partajare la distanță care se montează după ce containerul pornește (de exemplu sub `/mnt/remotes`) devine vizibilă fără repornire.

Backup-urile seturilor de date ZFS au și ele nevoie de acest mod: gazda montează instantaneul unui set de date abia după ce containerul a pornit. Vezi [Seturi de date ZFS](zfs-datasets.md).

Căile depozitelor de backup sunt implicit `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, create la primul backup. Schimbă locația oricând în **Setări, Stocare, Căi de backup**. Fiecare câmp de cale are și un comutator **Local / La distanță** integrat: o cale poate fi un remote restic (`s3:...`, `rest:...`, `sftp:...`, `rclone:...`) în loc de un folder local, iar backupul merge direct acolo, fără o copie locală separată; vezi [Depozite primare la distanță](offsite-recovery.md#remote-primary-repositories).

!!! note "Verificare integrare gazdă"
    Deschide `/spike` în interfața web după ce containerul pornește. Sondează fiecare montare și CLI (socket Docker, libvirt, restic, qemu-img, rclone) și raportează orice element lipsă.

## Detectarea surselor de copie de rezervă {#backup-source-detection}

Pentru fiecare container, BombVault alege singur ce montări bind și ce volume denumite intră în copie. O cale este preluată de îndată ce se aplică oricare dintre punctele următoare (rezultatul poate fi oricând suprascris per container, în secțiunea **Foldere de salvat** a acestuia):

- **Potrivire cu un segment al rădăcinii de date:** sursa de pe gazdă a montării bind conține unul dintre segmentele din `DATA_ROOT_SEGMENTS` ca element complet de cale (implicit doar `appdata`).
- **Volumele Docker denumite** sunt incluse întotdeauna, pentru că nu au un echivalent de unică folosință și deci nu e nimic de filtrat, **dar numai atunci când calea reală de stocare a volumului pe gazdă este ea însăși accesibilă prin montarea Host Data**, exact ca orice altă cale de gazdă pe care BombVault o salvează. Driverul implicit pentru volume locale așază un volum sub rădăcina de date a demonului însuși, adică `/var/lib/docker/volumes/<nume>/_data` dacă nu ai schimbat nimic (verifică cu `docker info -f '{{.DockerRootDir}}'`). Acel loc NU este acoperit de montarea Host Data îngustă, cu un singur director, pe care fișierul `docker-compose.yml` generic o folosește implicit. Un volum inaccesibil este sărit în tăcere, nu este o eroare. Ca volumele denumite să fie salvate cu adevărat pe o gazdă generică, îndreaptă Host Data (și `HOST_SOURCE_ROOT`) către un director părinte comun care acoperă și rădăcina de date a Docker: compromisul este descris în comentariul Host Data din fișierul compose (Unraid ocolește asta montând, din același motiv, întregul `/mnt`, propria sa convenție universală de nivel superior).
- **Directorul de proiect Docker Compose:** dacă containerul poartă eticheta obișnuită `com.docker.compose.project.working_dir` (pusă automat de `docker compose up`), și acel director este adăugat, indiferent dacă vreo montare bind a potrivit un segment al rădăcinii de date.
- **Suprascriere prin eticheta `bombvault.data`:** pune pe container eticheta `bombvault.data=true` pentru a include TOATE montările sale bind, pentru o organizare pe care niciuna dintre cele două convenții de mai sus nu o prinde (de exemplu o singură montare `/srv/plex/config` fără proiect Compose). Orice valoare nevidă în afară de `false` contează ca adevărată; o etichetă absentă sau `bombvault.data=false` nu schimbă nimic.
- **Eticheta `bombvault.dbdump`:** pune `bombvault.dbdump=false` pe un container pentru a-i opri dumpul automat al bazei de date (`0`, `no` și `off` fac același lucru), sau numește motorul (`postgres`, `mysql`, `mariadb`) ca să dumpezi un container pe care BombVault nu îl recunoaște singur. Eticheta are întâietate față de comutatorul de pe cardul containerului, care pe Unraid este calea obișnuită.

## Model de securitate

!!! warning "Control al gazdei echivalent cu root"
    Prin socket-ul Docker, BombVault poate opri, elimina și recrea containere și poate citi/scrie appdata, iar pentru backupul VM se autentifică pe gazdă prin SSH (`qemu+ssh://`, root implicit) pentru a rula `virsh`. Oricine poate ajunge la interfața sa web are efectiv root pe gazdă.

- **Protecție opțională cu parolă** (Setări, Securitate): setează o parolă pentru a cere autentificare, șterge-o pentru a dezactiva. Dezactivată implicit pentru utilizarea într-o rețea locală de încredere. Parola este stocată cu Argon2id peste o valoare condimentată cu `APP_KEY`, așa că un `/config` copiat nu valorează nimic fără cheie și este lent de atacat cu ea. O parolă nouă are nevoie de cel puțin 12 caractere; una mai scurtă deja existentă funcționează până este schimbată. Sesiunile sunt semnate (HMAC derivat din `APP_KEY`), iar schimbarea parolei le invalidează; autentificările sunt limitate la cinci eșecuri pe minut per client.
- **Autentificare în doi pași** (Setări): un cod temporal dintr-o aplicație de autentificare pe lângă parolă, plus opt coduri de recuperare de unică folosință, oferite o singură dată la activare. Secretul comun este stocat criptat cu `APP_KEY`, iar dezactivarea cere un cod curent.
- Deoarece bariera este opțională, când nu este setată, întreaga interfață și API (inclusiv configurarea off-site, rutele de test de manipulare și kitul de recuperare) sunt accesibile oricui poate ajunge la port. Activează bariera odată ce sunt folosite backupuri off-site, imuabile sau criptarea.
- Rulează BombVault doar într-o rețea de încredere, neexpusă. Pentru acces la distanță, pune-l în spatele unui reverse proxy care adaugă autentificare și TLS. Răspunsurile poartă anteturi de securitate de bază (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- În spatele unui proxy invers fiecare cerere poartă adresa proxy-ului, deci fără `TRUSTED_PROXY` limitarea numără toți clienții în aceeași găleată, iar eșecurile unui atacator te blochează și pe tine. Indică proxy-ul în `TRUSTED_PROXY` pentru a reveni la numărarea pe client.
- Un proxy invers în fața BombVault trebuie să transmită antetul `Authorization` sau `X-API-Key` către `/mcp` și nu are voie să pună răspunsurile în buffer, altfel asistenții nu se pot conecta. Vezi [Server MCP](mcp.md#tls).
- Cu `HTTP_ONLY=true` cookie-ul de sesiune își pierde indicatorul `Secure` (trebuie, ca să funcționeze peste HTTP simplu), deci activează parola în spatele unui proxy care termină TLS doar dacă confidențialitatea contează.
- Conexiunea SSH de backup VM are încredere în cheia gazdei la prima conexiune (TOFU) și o fixează ulterior. Verifică cheia gazdei prin alt canal dacă drumul container-către-gazdă nu este de încredere.
- Backupurile sunt criptate de restic când criptarea este activată (Setări; activată implicit), cu cheia derivată din `APP_KEY`.

## Server MCP {#mcp-server}

Serverul MCP nu are nevoie de nicio variabilă de mediu. Îl pornești creând o cheie în **Setări, Integrări, Server MCP**, iar el răspunde la `/mcp` pe același port ca interfața web (de exemplu `https://192.168.1.10:3443/mcp`). Fără o cheie activă, calea răspunde `404`. Clienții, certificatele și limitele sunt descrise în [Server MCP](mcp.md).

## Backup VM prin SSH

BombVault face backup VM-urilor KVM/libvirt **fără a monta vreo cale libvirt**. Rulează `virsh` pe gazdă prin SSH (`qemu+ssh://`), deci nu poate afecta niciodată VM Manager-ul gazdei tale.

Configurare rapidă:

1. **Setări, Integrări, SSH al gazdei:** copiază cheia publică afișată.
2. Adaug-o la `/root/.ssh/authorized_keys` al Unraid (persistată de asemenea în flash astfel încât să supraviețuiască reporniri).
3. Apasă **Testează conexiunea**.

Șablonul adaugă `--add-host=host.docker.internal:host-gateway` astfel încât containerul să poată ajunge la gazdă. Setează `LIBVIRT_HOST` la IP-ul LAN al Unraid dacă acel nume nu se rezolvă (de exemplu când containerul rulează pe o rețea `br0.x` personalizată). Dacă ai schimbat portul SSH al Unraid, setează `LIBVIRT_SSH_PORT` să corespundă. **Instantaneele live** au nevoie suplimentar de qemu guest agent în VM și de discul pe `/mnt/cache` (nu `/mnt/user`).

!!! important "Ghid complet de configurare și rețea pentru VM"
    Ghidul complet pas cu pas (activarea SSH, autorizarea persistentă a cheii, rutarea în rețea personalizată și VLAN, metoda per VM și depanarea pe partea de gazdă) se află la [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) pe GitHub.

## Configurare off-site

Configurează o replică off-site în pagina **Setări, Extern**. Vezi [Off-site și recuperare](offsite-recovery.md) pentru fluxul complet (imuabil/append-only, testarea manipulării și exercițiile DR). Pe scurt:

- **Backenduri:** SMB/CIFS și NFS (montează partajarea și îndreaptă o cale de backup către ea), backenduri restic native fără rclone (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`) sau orice remote rclone (`rclone:<remote>:<bucket>/path`). Backblaze B2 nu are aici un backend nativ: se accesează prin punctul său final S3 (`s3:https://s3.<region>.backblazeb2.com/<bucket>/<path>`), cu ID-ul cheii și cheia aplicației ca acreditări S3.
- **Credențialele cloud partajate** sunt stocate criptat sub Setări, Acces cloud, Credențiale cloud partajate.
- **Țintele SSH nu necesită nimic instalat pe partea îndepărtată.** `sftp:` necesită doar un server SSH. Adaugă cheia publică din **Setări, Integrări, SSH al gazdei** (de asemenea la `/config/ssh/id_ed25519.pub`) la `~/.ssh/authorized_keys` al utilizatorului țintă.
- **Copie off-site:** BombVault replică instantaneele noi cu `restic copy` pe bază de best-effort, pe lângă un depozit primar (de obicei local). Fiecare domeniu are propria programare off-site, plus un buton **Replică acum**.
- **Mai multe ținte off-site per domeniu:** fiecare domeniu poate replica către mai multe destinații off-site simultan. Adaugă ținte suplimentare în Setări, Extern, fiecare cu propriul depozit, clasă de stocare S3, indicator append-only, retenție și buget de creștere; toate replică conform programării off-site a acelui domeniu. O configurare off-site unică existentă este preluată ca prima țintă.
- **Retenție per sursă:** politicile locală și off-site se află ambele în Setări, Retenție (las-o pe cea off-site toată zero pentru a nu tăia niciodată automat instantaneele off-site). Cardurile **Retenție locală** și **Retenție externă** au fiecare **Reguli de retenție pe sursă**, care le dă containerelor, VM-urilor, flash-ului, folderelor, ZFS sau auto-backupului reguli de retenție proprii, pentru backupurile lor locale și pentru repo-ul lor off-site. O sursă fără reguli proprii le urmează pe cele comune, iar retenția după fiecare backup, copia off-site, o curățare manuală și previzualizarea retenției folosesc toate regulile sursei în cauză. Țintele off-site suplimentare își păstrează regulile setate pentru ele în Setări, Extern.
- **Limite de lățime de bandă:** limitează rata de upload/download restic sub Setări, Extern.
- **Întâi streamingul:** în Setări, Extern, alegi serverele media (Plex, Jellyfin și Emby sunt preselectate după numele imaginii), rata de trimitere de la care unul contează ca făcând streaming, limita de încărcare în timpul streamingului și după cât timp de la un stream revine limita normală.
- **Clasă de stocare la rece și de arhivă (S3):** pentru un depozit off-site S3 nativ, alege un nivel care permite restaurarea (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). Remote-urile rclone își setează clasa în configurația rclone.
- **Primar la distanță în loc de local:** calea de backup a unui domeniu poate fi ea însăși unul dintre backendurile de mai sus, fără copie locală și fără pas de replicare; vezi [Depozite primare la distanță](offsite-recovery.md#remote-primary-repositories) pentru comutatorul Local/La distanță integrat și setările lui de siguranță (lățime de bandă, append-only, buget de creștere).

## Anomalii {#anomalies}

Detecția anomaliilor se configurează în cardul **Anomalii** din **Setări, Integritate**. Fiecare control se salvează imediat ce îl schimbi, iar cele trei de sub comutator sunt ascunse cât timp detecția este oprită.

| Setare | Implicit | Ce face |
|---|---|---|
| **Detectează anomalii** | Pornit | Compară fiecare backup cu istoricul propriu al elementului. Oprit, nu se mai verifică nimic nou și intrarea **Anomalii** dispare din bara laterală; cardul trimite în continuare la constatările anterioare. |
| **Sensibilitate** | Echilibrată | Strictă raportează schimbări mai mici, Permisivă doar pe cele mari. |
| **Trimite o notificare pentru** | Doar constatări critice | Gravitatea minimă care trimite un mesaj prin canalele configurate în Notificări. Eșecurile repetate ale backupurilor și dumpurilor și verificările de restaurare programate eșuate trimit deja propriul mesaj și nu sunt trimise de două ori. |
| **Păstrează copiile vechi când o sursă se micșorează brusc sau este rescrisă** | Pornit | Cât timp un element are o constatare deschisă pentru o sursă aproape goală, o micșorare puternică sau cea mai mare parte a datelor salvată din nou, retenția și curățarea lasă în pace backupurile lui vechi. Confirmă constatarea sau marcheaz-o ca așteptată ca să le eliberezi. |

Fiecare element poate avea propria sensibilitate și propriul minim de notificare. Setează-le pe pagina **Anomalii**, unde un element cu constatări deschise le are la **Monitorizare** pe cardul lui, iar orice alt element le deschide din cardul **Nimic deschis**, sau în panoul elementului: secțiunea de foldere a unui container și setările unei VM (ambele în modul avansat), editorul de foldere al unui set de foldere și paginile **Flash** și **Auto-backup**. Pentru un element ZFS se află în editorul lui de pe pagina **ZFS** și se aplică fiecărui set de date din arborele lui.

## Setări portabile (export și import) {#portable-settings-export-and-import}

Cardul **Exportă / importă setările** de pe pagina Setări, Sistem scrie întreaga ta configurație BombVault (setări de domeniu, ținte off-site, programări, retenție, notificări) într-un fișier JSON portabil pe care îl poți importa pe o altă instanță, astfel încât mutarea pe o stație nouă sau clonarea unei configurații să nu însemne reintroducerea totul manual. Importul arată o previzualizare și cere confirmare și nu îți atinge niciodată datele sau istoricul de backup.

!!! warning "Exportul poate conține credențiale"
    Alegi dacă incluzi credențialele off-site, de notificare și ale brokerului MQTT în fișier. Cu credențialele incluse, exportul este la fel de sensibil ca kitul tău de recuperare, deci păstrează-l undeva în siguranță. Fără ele, fișierul conține doar setări nesecrete.
