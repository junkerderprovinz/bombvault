# Configurare

Această pagină acoperă variabilele de mediu ale containerului, montările pe care le oferă șablonul, backupul VM prin SSH și configurarea off-site. Stabilești unde ajung backupurile în interiorul aplicației, în **Setări, Stocare**, nu prin variabile de mediu.

## Variabile de mediu

| Variabilă | Obligatorie | Descriere |
|---|---|---|
| `APP_KEY` | **Da** | Secret hex de 32 de octeți (64 de caractere hex) folosit pentru a deriva parola depozitului restic. Generează cu `openssl rand -hex 32`. Păstrează-l în siguranță: pierderea lui face ca backupurile criptate să nu mai poată fi recuperate. |
| `LIBVIRT_HOST` | Pentru VM-uri | Gazda Unraid accesată prin SSH pentru backupul VM (implicit `host.docker.internal`; șablonul precompletează un placeholder cu IP-LAN). Folosește IP-ul LAN al Unraid, obligatoriu pe o rețea `br0.x` personalizată. Folosit și pentru backup-urile seturilor de date ZFS (câmpul de șablon **Host SSH: Address**); substituentul `192.168.x.x` contează ca nesetat. |
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
| `BACKUP_STALL_HOURS` | Nu | Orele în care un backup poate să nu facă **niciun progres** înainte de a fi anulat. Gol (implicitul) folosește `2`; `0` nu anulează niciodată pentru blocare. Este cea mai fină dintre cele două gărzi și de obicei cea care se declanșează: urmărește dacă se mai întâmplă ceva, nu cât durează rularea, așa că un backup de mai mulți terabytes lent dar sănătos este lăsat în pace, iar unul blocat pe o partajare care nu răspunde este oprit în ore în loc de zile. După 30 de minute de liniște se înregistrează un avertisment, înainte de a anula ceva. Scanarea contează drept progres: restic nu scrie niciun byte cât parcurge un arbore mare, iar faza aceasta este urmărită prin totalurile ei de fișiere și bytes, nu prin bytes scriși. Cele două variabile sunt independente, iar `BACKUP_MAX_HOURS` limitează în continuare fazele de după backupul propriu-zis (retenție, statistici, copie off-site), unde nu există contoare de urmărit. |
| `DB_DUMP_MAX_HOURS` | Nu | Orele în care un dump automat de bază de date poate rula înainte să fie oprit. Gol (implicit) înseamnă `6`; sunt permise valori de la `1` la `48`, iar limita rămâne cu o oră sub `BACKUP_MAX_HOURS` (la jumătatea acesteia când e sub două ore), astfel încât un dump lung să fie tăiat de propria limită și raportat ca atare, în loc să tragă backupul după el. Un dump care nu mai avansează este oprit mai devreme, după `BACKUP_STALL_HOURS`. Un dump oprit eșuează de unul singur, iar backupul containerului continuă. Pe Unraid adaugi variabila la containerul BombVault cu **Add another Path, Port, Variable**. |
| `TZ` | Nu | Fusul orar pentru programator (de exemplu `Europe/Berlin`). **Dacă nu este setată, toate programările rulează în UTC**: o programare la 02:30 pornește atunci la 02:30 UTC, nu la ora locală. Pe Unraid nu setați niciodată acest lucru: sistemul transmite propriul fus orar fiecărui container. |

## Montări

Montează socket-ul Docker, flash-ul (`/boot`) și rădăcina **Host Data** (`/mnt`) așa cum se arată în șablonul CA. Atât *sursele* cât și *destinațiile* backupurilor se află sub Host Data, iar aceasta este montată **slave** astfel încât o partajare la distanță care se montează după ce containerul pornește (de exemplu sub `/mnt/remotes`) devine vizibilă fără repornire.

Backup-urile seturilor de date ZFS au și ele nevoie de acest mod: gazda montează instantaneul unui set de date abia după ce containerul a pornit. Vezi [Seturi de date ZFS](zfs-datasets.md).

O instalare nouă stochează fiecare domeniu în locul **Unraid**, la `/mnt/user/bombvault`, cu câte un folder pentru fiecare domeniu (`container`, `vms`, `flash`, `config`, `files`, `zfs`), creat la primul backup. Alte locuri, locale sau la distanță, le adaugi în **Setări, Stocare**; vezi [Locuri de stocare](storage-places.md).

!!! note "Verificarea integrării cu gazda"
    Deschide `/spike` în interfața web după ce containerul pornește. Sondează fiecare montare și CLI (socket Docker, libvirt, restic, qemu-img, rclone) și raportează orice element lipsă.

## Detectarea surselor de copie de rezervă {#backup-source-detection}

Pentru fiecare container, BombVault alege singur ce montări bind și ce volume denumite intră în copie. O cale este preluată de îndată ce se aplică oricare dintre punctele următoare (rezultatul poate fi oricând suprascris per container, în **Căile de copiere** ale acestuia):

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

Serverul MCP nu are nevoie de nicio variabilă de mediu. Îl pornești creând o cheie în **Setări, Sistem, Server MCP**, iar el răspunde la `/mcp` pe același port ca interfața web (de exemplu `https://192.168.1.10:3443/mcp`). Fără o cheie activă, calea răspunde `404`. Clienții, certificatele și limitele sunt descrise în [Server MCP](mcp.md).

## Backup VM prin SSH

BombVault face backup VM-urilor KVM/libvirt **fără a monta vreo cale libvirt**. Rulează `virsh` pe gazdă prin SSH (`qemu+ssh://`), deci nu poate afecta niciodată VM Manager-ul gazdei tale.

Configurare rapidă:

1. **Setări, Sistem, SSH al gazdei:** copiază cheia publică afișată.
2. Adaug-o la `/root/.ssh/authorized_keys` al Unraid (persistată de asemenea în flash astfel încât să supraviețuiască reporniri).
3. Apasă **Test connection**.

Șablonul adaugă `--add-host=host.docker.internal:host-gateway` astfel încât containerul să poată ajunge la gazdă. Setează `LIBVIRT_HOST` la IP-ul LAN al Unraid dacă acel nume nu se rezolvă (de exemplu când containerul rulează pe o rețea `br0.x` personalizată). Dacă ai schimbat portul SSH al Unraid, setează `LIBVIRT_SSH_PORT` să corespundă. **Instantaneele live** au nevoie suplimentar de qemu guest agent în VM și de discul pe `/mnt/cache` (nu `/mnt/user`).

!!! important "Ghid complet de configurare și rețea pentru VM"
    Ghidul complet pas cu pas (activarea SSH, autorizarea persistentă a cheii, rutarea în rețea personalizată și VLAN, metoda per VM și depanarea pe partea de gazdă) se află la [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) pe GitHub.

## Configurare off-site

Copiile off-site ajung în locuri de stocare. Adaugă locul în **Setări, Stocare** cu **Adaugă loc**, apoi bifează-l la **Copiat în** pe rândul domeniului. [Locuri de stocare](storage-places.md) acoperă fiecare tip de conexiune, iar [Off-site și recuperare](offsite-recovery.md) acoperă append-only, testarea manipulării și exercițiile DR. Pe scurt:

- **Tipuri de conexiune:** un folder pe acest Unraid sau o partajare NAS montată sub `/mnt/remotes`, stocare S3 (Backblaze B2 și ceilalți furnizori de cloud, sau un serviciu găzduit de tine, precum MinIO sau Garage), rest-server, SFTP inclusiv un Hetzner Storage Box, WebDAV pentru Nextcloud, ownCloud și OpenCloud, Azure Blob și orice remote rclone. Backblaze B2 are nevoie doar de cheie: BombVault citește din ea bucket-ul și endpointul S3.
- **Credențialele** sunt stocate criptat, împreună cu locul căruia îi aparțin. Seturile de credențiale folosite de sursele de preluare se află în fila **Preluare** a paginii Instanțe.
- **Țintele SSH nu necesită nimic instalat pe partea îndepărtată.** Un loc SFTP necesită doar un server SSH. Adaugă cheia publică afișată în formularul SFTP (o găsești și în **Setări, Sistem, Backup VM prin SSH** și la `/config/ssh/id_ed25519.pub`) la `~/.ssh/authorized_keys` al utilizatorului țintă.
- **Copie off-site:** BombVault copiază instantaneele noi cu `restic copy` pe bază de best-effort, în plus față de locul în care este stocat domeniul. Fiecare domeniu are propria programare de copiere în Setări, Programări, plus **Copiază acum** pe rândul lui.
- **Mai multe locuri de copiere per domeniu:** bifează la **Copiat în** oricâte locuri vrei; fiecare copiază conform programării domeniului.
- **Retenția, limitele, clasa de stocare și bugetul de creștere țin de loc** și se setează în detaliile lui. Retenția unui loc se aplică fiecărui depozit din el, așa că un loc off-site poate păstra copiile mai mult timp ca arhivă; un loc cu toate regulile la zero nu curăță niciodată nimic.
- **Clasă de stocare la rece și de arhivă (S3):** pentru un loc S3, alege un nivel care permite restaurarea (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). Remote-urile rclone își setează clasa în configurația rclone.
- **Un domeniu stocat într-un loc la distanță:** vezi [Un domeniu stocat într-un loc la distanță](offsite-recovery.md#remote-primary-repositories).

## Anomalii {#anomalies}

Detecția anomaliilor se configurează în cardul **Anomalii** din **Setări, Integritate**. Fiecare control se salvează imediat ce îl schimbi, iar cele trei de sub comutator sunt ascunse cât timp detecția este oprită.

| Setare | Implicit | Ce face |
|---|---|---|
| **Detectează anomalii** | Pornit | Compară fiecare backup cu istoricul propriu al elementului. Oprit, nu se mai verifică nimic nou și intrarea **Anomalii** dispare din bara laterală; cardul trimite în continuare la constatările anterioare. |
| **Sensibilitate** | Echilibrată | Strictă raportează schimbări mai mici, Permisivă doar pe cele mari. |
| **Trimite o notificare pentru** | Doar constatări critice | Gravitatea minimă care trimite un mesaj prin canalele configurate în Notificări. Eșecurile repetate ale backupurilor și dumpurilor și verificările de restaurare programate eșuate trimit deja propriul mesaj și nu sunt trimise de două ori. |
| **Păstrează copiile vechi când o sursă se micșorează brusc sau este rescrisă** | Pornit | Cât timp un element are o constatare deschisă pentru o sursă aproape goală, o micșorare puternică sau cea mai mare parte a datelor salvată din nou, retenția și curățarea lasă în pace backupurile lui vechi. Confirmă constatarea sau marcheaz-o ca așteptată ca să le eliberezi. |

Fiecare element poate avea propria sensibilitate și propriul minim de notificare. Setează-le în fila **Elemente** a paginii **Anomalii** sau în panoul elementului: secțiunea de foldere a unui container și setările unei VM (ambele în modul avansat), editorul de foldere al unui set de foldere și paginile **Flash** și **Auto-backup**. Pentru un element ZFS se află în editorul lui de pe pagina **ZFS** și se aplică fiecărui set de date din arborele lui.

## Setări portabile (export și import) {#portable-settings-export-and-import}

Cardul **Export și import setări** de pe pagina Setări scrie întreaga ta configurație BombVault (setări de domeniu, locuri de stocare, programări, notificări) într-un fișier JSON portabil pe care îl poți importa pe o altă instanță, astfel încât mutarea pe o stație nouă sau clonarea unei configurații să nu însemne reintroducerea totul manual. Importul arată o previzualizare și cere confirmare și nu îți atinge niciodată datele sau istoricul de backup. Previzualizarea numără locurile de stocare din fișier și, împreună cu credențialele, seturile de credențiale; pentru un fișier mai vechi fără locuri, BombVault le construiește din setările importate.

!!! warning "Exportul poate conține credențiale"
    Alegi dacă incluzi în fișier credențialele locurilor și ale notificărilor tale. Cu credențialele incluse, exportul este la fel de sensibil ca kitul tău de recuperare, deci păstrează-l undeva în siguranță. Fără ele, fișierul conține doar setări nesecrete.
