# Configurare

Această pagină acoperă variabilele de mediu ale containerului, montările pe care le oferă șablonul, backupul VM prin SSH și configurarea off-site. **Căile depozitelor** de backup sunt configurate în interiorul aplicației (Setări, Căi de backup), nu prin variabile de mediu.

## Variabile de mediu

| Variabilă | Obligatorie | Descriere |
|---|---|---|
| `APP_KEY` | **Da** | Secret hex de 32 de octeți (64 de caractere hex) folosit pentru a deriva parola depozitului restic. Generează cu `openssl rand -hex 32`. Păstrează-l în siguranță: pierderea lui face ca backupurile criptate să nu mai poată fi recuperate. |
| `LIBVIRT_HOST` | Pentru VM-uri | Gazda Unraid accesată prin SSH pentru backupul VM (implicit `host.docker.internal`; șablonul precompletează un placeholder cu IP-LAN). Folosește IP-ul LAN al Unraid, obligatoriu pe o rețea `br0.x` personalizată. |
| `LIBVIRT_SSH_PORT` | Nu | Portul SSH al gazdei pentru backupul VM (implicit `22`). |
| `LIBVIRT_SSH_USER` | Nu | Utilizatorul SSH de pe gazdă pentru backupul VM (implicit `root`). |
| `PORT` | Nu | Portul HTTP (implicit `3000`; folosit doar cu `HTTP_ONLY=true`). |
| `HTTPS_PORT` | Nu | Portul HTTPS (implicit `3443`; șablonul îl publică 1:1, deci WebUI răspunde la `https://<ip>:3443`). |
| `HTTP_ONLY` | Nu | Setează `true` pentru a dezactiva ascultătorul HTTPS auto-semnat și a servi doar HTTP simplu (pentru utilizare în spatele unui reverse proxy care termină TLS). |
| `HOST_SOURCE_ROOT` | Nu | Calea gazdei montată ca **Host Data** (implicit `/mnt`). BombVault traduce sursele de bind-mount raportate de Docker în căi sub această montare. Schimbă doar dacă ai montat o altă rădăcină de gazdă. |
| `BOMBVAULT_SELF_CONTAINER` | Nu | Numele containerului BombVault însuși, astfel încât să nu-și facă niciodată backup (și deci să nu se oprească) singur (implicit `BombVault`; detectat automat prin hostname pe rețea bridge). |
| `BACKUP_MAX_HOURS` | Nu | Numărul maxim de ore de ceas pe care o singură rulare de backup îl poate ține blocajul de domeniu înainte de a fi forțat anulată (o gardă astfel încât o rulare blocată să nu poată bloca domeniul la nesfârșit). Gol (implicitul) folosește `48`. Ridică-l pentru backupuri cloud foarte mari sau lente (o rulare anulată la limită eșuează cu `context deadline exceeded`). Setează `0` pentru a dezactiva complet limita. |
| `TZ` | Nu | Fusul orar pentru programator (de exemplu `Europe/Berlin`). |

## Montări

Montează socket-ul Docker, flash-ul (`/boot`) și rădăcina **Host Data** (`/mnt`) așa cum se arată în șablonul CA. Atât *sursele* cât și *destinațiile* backupurilor se află sub Host Data, iar aceasta este montată **rslave** astfel încât o partajare la distanță care se montează după ce containerul pornește (de exemplu sub `/mnt/remotes`) devine vizibilă fără repornire.

Căile depozitelor de backup sunt implicit `/mnt/user/bombvault/{container,vms,flash,config,files}`, create la primul backup. Schimbă locația oricând în **Setări, Căi de backup**.

!!! note "Verificarea integrării cu gazda"
    Deschide `/spike` în interfața web după ce containerul pornește. Sondează fiecare montare și CLI (socket Docker, libvirt, restic, qemu-img, rclone) și raportează orice element lipsă.

## Model de securitate

!!! warning "Control al gazdei echivalent cu root"
    Prin socket-ul Docker, BombVault poate opri, elimina și recrea containere și poate citi/scrie appdata, iar pentru backupul VM se autentifică pe gazdă prin SSH (`qemu+ssh://`, root implicit) pentru a rula `virsh`. Oricine poate ajunge la interfața sa web are efectiv root pe gazdă.

- **Protecție opțională cu parolă** (Setări, Securitate): setează o parolă pentru a cere autentificare, șterge-o pentru a dezactiva. Oprită implicit pentru utilizare pe LAN de încredere. Sesiunile sunt semnate (HMAC derivat din `APP_KEY`) și schimbarea parolei le invalidează; autentificările sunt limitate ca rată.
- Deoarece bariera este opțională, când nu este setată, întreaga interfață și API (inclusiv configurarea off-site, rutele de test de manipulare și kitul de recuperare) sunt accesibile oricui poate ajunge la port. Activează bariera odată ce sunt folosite backupuri off-site, imuabile sau criptarea.
- Rulează BombVault doar într-o rețea de încredere, neexpusă. Pentru acces la distanță, pune-l în spatele unui reverse proxy care adaugă autentificare și TLS. Răspunsurile poartă anteturi de securitate de bază (CSP, `nosniff`, `X-Frame-Options`, `Referrer-Policy`).
- Cu `HTTP_ONLY=true` cookie-ul de sesiune își pierde indicatorul `Secure` (trebuie, ca să funcționeze peste HTTP simplu), deci activează parola în spatele unui proxy care termină TLS doar dacă confidențialitatea contează.
- Conexiunea SSH de backup VM are încredere în cheia gazdei la prima conexiune (TOFU) și o fixează ulterior. Verifică cheia gazdei prin alt canal dacă drumul container-către-gazdă nu este de încredere.
- Backupurile sunt criptate de restic când criptarea este activată (Setări; activată implicit), cu cheia derivată din `APP_KEY`.

## Backup VM prin SSH

BombVault face backup VM-urilor KVM/libvirt **fără a monta vreo cale libvirt**. Rulează `virsh` pe gazdă prin SSH (`qemu+ssh://`), deci nu poate afecta niciodată VM Manager-ul gazdei tale.

Configurare rapidă:

1. **Setări, Sistem, VM Backup over SSH:** copiază cheia publică afișată.
2. Adaug-o la `/root/.ssh/authorized_keys` al Unraid (persistată de asemenea în flash astfel încât să supraviețuiască reporniri).
3. Apasă **Test connection**.

Șablonul adaugă `--add-host=host.docker.internal:host-gateway` astfel încât containerul să poată ajunge la gazdă. Setează `LIBVIRT_HOST` la IP-ul LAN al Unraid dacă acel nume nu se rezolvă (de exemplu când containerul rulează pe o rețea `br0.x` personalizată). Dacă ai schimbat portul SSH al Unraid, setează `LIBVIRT_SSH_PORT` să corespundă. **Instantaneele live** au nevoie suplimentar de qemu guest agent în VM și de discul pe `/mnt/cache` (nu `/mnt/user`).

!!! important "Ghid complet de configurare și rețea pentru VM"
    Ghidul complet pas cu pas (activarea SSH, autorizarea persistentă a cheii, rutarea în rețea personalizată și VLAN, metoda per VM și depanarea pe partea de gazdă) se află la [docs/vm-backup-ssh-setup.md](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) pe GitHub.

## Configurare off-site

Configurează o replică off-site în fila **Setări, Off-site**. Vezi [Off-site și recuperare](offsite-recovery.md) pentru fluxul complet (imuabil/append-only, testarea manipulării și exercițiile DR). Pe scurt:

- **Backenduri:** SMB/CIFS și NFS (montează partajarea și îndreaptă o cale de backup către ea), backenduri restic native fără rclone (`s3:...`, `rest:http://host:8000/repo`, `b2:...`, `sftp:user@host:/repo`) sau orice remote rclone (`rclone:<remote>:<bucket>/path`).
- **Credențialele cloud** sunt stocate criptat sub Setări, Off-site, Credențiale cloud.
- **Țintele SSH nu necesită nimic instalat pe partea îndepărtată.** `sftp:` necesită doar un server SSH. Adaugă cheia publică din **Setări, Sistem, VM Backup over SSH** (de asemenea la `/config/ssh/id_ed25519.pub`) la `~/.ssh/authorized_keys` al utilizatorului țintă.
- **Copie off-site:** BombVault replică instantaneele noi cu `restic copy` pe bază de best-effort. Depozitul local rămâne principal. Fiecare domeniu are propria programare off-site, plus un buton **Replicate now**.
- **Mai multe ținte off-site per domeniu:** fiecare domeniu poate replica către mai multe destinații off-site simultan. Adaugă ținte suplimentare în Setări, Off-site, fiecare cu propriul depozit, clasă de stocare S3, indicator append-only, retenție și buget de creștere; toate replică conform programării off-site a acelui domeniu. O configurare off-site unică existentă este preluată ca prima țintă.
- **Retenție per sursă:** politica locală se află în Setări, Căi și Stocare; politica off-site în Setări, Off-site (las-o toată zero pentru a nu tăia niciodată automat instantaneele off-site).
- **Limite de lățime de bandă:** limitează rata de upload/download restic sub Setări, Off-site.
- **Clasă de stocare la rece și de arhivă (S3):** pentru un depozit off-site S3 nativ, alege un nivel care permite restaurarea (Standard, Standard-IA, One Zone-IA, Intelligent-Tiering, Glacier Instant Retrieval). Remote-urile rclone își setează clasa în configurația rclone.

## Setări portabile (export și import) {#portable-settings-export-and-import}

Cardul **Export și import setări** de pe pagina Setări scrie întreaga ta configurație BombVault (setări de domeniu, ținte off-site, programări, retenție, notificări) într-un fișier JSON portabil pe care îl poți importa pe o altă instanță, astfel încât mutarea pe o stație nouă sau clonarea unei configurații să nu însemne reintroducerea totul manual. Importul arată o previzualizare și cere confirmare și nu îți atinge niciodată datele sau istoricul de backup.

!!! warning "Exportul poate conține credențiale"
    Alegi dacă incluzi credențialele off-site și de notificare în fișier. Cu credențialele incluse, exportul este la fel de sensibil ca kitul tău de recuperare, deci păstrează-l undeva în siguranță. Fără ele, fișierul conține doar setări nesecrete.
