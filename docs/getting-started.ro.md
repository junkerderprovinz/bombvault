# Primii pași

Această pagină te conduce de la o stație Unraid nouă până la primul tău backup.

## Cerințe {#requirements}

| Cerință | Note |
|---|---|
| **Unraid 6.12+** | Versiunile mai vechi nu sunt testate. Unraid este ținta principală, dar BombVault rulează și pe o gazdă Docker obișnuită și pe TrueNAS Scale (vezi [Gazdă Docker generică](#generic-docker-host) și [TrueNAS SCALE](#truenas)). |
| **Locația depozitului restic** | O cale locală (recomandat: array-ul sau cache-ul tău), SMB, NFS sau orice backend rclone. |
| **Socket Docker** | Montat automat de șablon (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Montat integral de șablon, automat (`/boot` la `/host/boot`). Alimentează backupul flash și permite ca un container restaurat să reapară ca o aplicație Unraid normală, editabilă. |
| **VM-uri KVM** (opțional) | Backupul VM comunică cu libvirt prin SSH, fără montare libvirt. Configurează-l la **Setări, Integrări, SSH al gazdei** (vezi [Configurare](configuration.md)). |
| **Seturi de date ZFS** (opțional) | Aceeași legătură SSH ca la backupul VM, `zfs` pe gazdă și Host Data mapat ca `/mnt` cu modul de acces Read/Write - Slave, valoarea implicită a șablonului. Vezi [Seturi de date ZFS](zfs-datasets.md). |
| **Aplicația Android** (facultativ) | Android 10 sau mai nou, împerecheată cu servere la versiunea 9.7.0 sau mai nouă. Vezi [Aplicația Android](android.md). |

## Instalare pe Unraid {#install-on-unraid}

Calea cea mai simplă este prin **Community Applications**.

1. Deschide fila **Apps** în Unraid.
2. Caută **BombVault**.
3. Apasă **Install**, setează variabilele necesare (mai jos) și aplică.

!!! tip "Instalare manuală a șablonului"
    Dacă preferi să adaugi șablonul manual:

    1. Mergi la **Docker, Add Container, Template repositories** și adaugă:
       ```
       https://github.com/junkerderprovinz/unraid-apps
       ```
    2. Caută **BombVault** în Templates.
    3. Setează variabilele necesare și apasă **Apply**.

## Gazdă Docker generică {#generic-docker-host}

Nu ești pe Unraid? BombVault rulează și ca un container obișnuit pe orice gazdă Docker. TrueNAS SCALE are pașii lui proprii la [TrueNAS SCALE](#truenas).

1. Ia din depozit fișierul [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), gata de editat.
2. Setează `APP_KEY` (vezi mai jos) și îndreaptă volumul Host Data către rădăcina ta reală de date. Comentariile din fișier le parcurg pe amândouă.
3. `docker compose up -d`, apoi deschide `https://<ip-gazdă>:3443/`.

Ce este diferit față de Unraid:

- **Nu există secțiunea Flash.** Nu există un stick de pornire de capturat sau de restaurat, așa că secțiunea Flash nu are ce face aici. În schimb, când Host Data ajunge la `/etc` al gazdei, **Adaugă** de pe pagina Copii de rezervă oferă **Configurația sistemului** ca sugestie de folder: un set inițial de foldere `/etc` pe care îl revizuiești și îl editezi înainte de salvare.
- **Nu există notificări native Unraid.** Canalele proprii de notificare ale BombVault (webhook, alerte de eșec off-site și așa mai departe) funcționează ca de obicei. Se omite doar trimiterea specifică sistemului de notificări al Unraid, întrucât aici nu există un asemenea sistem.
- **Backupul VM este opțional și are nevoie de o gazdă libvirtd separată, accesibilă prin SSH.** Vezi blocul comentat din fișierul compose. O gazdă Docker generică nu vine cu niciun administrator de mașini virtuale.
- **Nu există widget pentru panou.** BombVault Widget este un plugin Unraid, așa că și acest pas este omis.
- **Găsirea datelor unui container.** Fără convenția `appdata` a Unraid, folderul de date al unui container este găsit pornind de la segmentele din `DATA_ROOT_SEGMENTS`, volumele Docker cu nume, directorul de lucru al unui proiect Compose și eticheta `bombvault.data` (vezi [Detectarea surselor de copie de rezervă](configuration.md#backup-source-detection)). Volumele cu nume și presetarea `/etc` ajung doar la căi din interiorul montării Host Data, așa că îndreaptă Host Data către un director părinte comun care acoperă și rădăcina de date a Docker.
- **`PLATFORM`.** Setează-l la `generic` sau `truenas`. Dacă nu este setat, BombVault recunoaște Unraid după propriul marcaj de pe montarea flash și tratează orice altceva ca generic, iar pașii specifici Unraid sunt omiși în loc să fie încercați și să eșueze.

### TrueNAS SCALE {#truenas}

BombVault nu este încă în catalogul de aplicații TrueNAS, așa că instalează-l ca aplicație personalizată: **Apps, Discover Apps**, apoi **Install via YAML** din meniul de lângă **Custom App**. YAML-ul de mai jos setează aceleași lucruri ca intrarea de catalog din folderul `truenas-apps` al depozitului. Înlocuiește `tank` cu pool-ul tău și completează `APP_KEY` așa cum descrie secțiunea următoare.

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    hostname: bombvault
    restart: unless-stopped
    cap_drop: [ALL]
    cap_add: [CHOWN, DAC_OVERRIDE, FOWNER, SETGID, SETPCAP, SETUID, SYS_CHROOT]
    environment:
      APP_KEY: ""
      PLATFORM: truenas
      HOST_SOURCE_ROOT: /mnt/tank
      HOST_MOUNT_ROOT: /mnt/tank
      # For VM backups:
      # LIBVIRT_URI: qemu+ssh://root@<truenas-ip>/system?socket=/run/truenas_libvirt/libvirt-sock
    volumes:
      - /mnt/tank/apps/bombvault:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - type: bind
        source: /mnt/tank
        target: /mnt/tank
        bind:
          propagation: rslave
    ports:
      - "30528:3443"
```

Apoi deschide `https://<truenas-ip>:30528/`. Ce este enumerat la [Ce este diferit față de Unraid](#generic-docker-host) se aplică și aici, iar pe TrueNAS:

- **Capabilități.** BombVault are nevoie de cele șapte de mai sus. Fără `SETPCAP`, backupurile VM eșuează, iar fără `SYS_CHROOT` fiecare container este oprit pe toată durata copiei sale în loc să fie citit dintr-un instantaneu.
- **Host Data** este o cale a gazdei pe un set de date, aici `/mnt/tank`, montată la aceeași cale în aplicație. De aceea `HOST_SOURCE_ROOT` și `HOST_MOUNT_ROOT` o numesc amândouă. `rslave` transmite montările pe care gazda le face mai târziu, de care au nevoie backupurile seturilor de date ZFS.
- **Datele dintr-un ixVolume nu intră în backup.** O aplicație al cărei spațiu de stocare este un ixVolume își ține datele sub `/mnt/.ix-apps`, în afara Host Data. BombVault salvează doar definiția aplicației, iar rularea numește folderele pe care le-a lăsat deoparte. Pentru fiecare aplicație ale cărei date vrei să le salvezi, alege stocare **Host Path** pe un set de date de sub Host Data.
- **Backupurile VM și ale seturilor de date ZFS** ajung la TrueNAS prin SSH ca root. Pornește serviciul SSH la **System, Services** și adaugă cheia publică din **Setări, Integrări, SSH al gazdei** la utilizatorul root, la **Credentials, Users**. Backupurile VM au nevoie și de `LIBVIRT_URI` ca mai sus, pentru că libvirtd-ul din TrueNAS ascultă pe un socket propriu. Dacă nu este setat nici `LIBVIRT_URI`, nici `LIBVIRT_HOST`, BombVault folosește ca gazdă gateway-ul implicit al aplicației, pentru că `host.docker.internal` nu se rezolvă într-o aplicație TrueNAS. [Ghidul de backup VM](vm-backup-ssh-setup.md#truenas-scale) acoperă detaliile, inclusiv VM-urile pe zvol-uri.
- **O replică ZFS folosește altă cheie.** O replică pe un al doilea server ZFS se conectează cu cheia pe care o arată dialogul replicii, nu cu cheia SSH a gazdei. Adaug-o pe aceea pe serverul care primește.
- **O aplicație ștearsă revine în doi pași.** Reinstaleaz-o mai întâi în TrueNAS cu același nume, ceea ce îi creează din nou rețeaua, apoi restaurează-i datele în BombVault. Până atunci, verificarea înainte de restaurare raportează rețeaua lipsă, iar restaurarea se oprește înainte să schimbe ceva.
- **Lângă original** creează un container Docker obișnuit. Rulează, dar ecranul Apps din TrueNAS nu îl afișează.
- **Dumpurile bazelor de date** funcționează pe montări de seturi de date. Un import într-un folder de date care este un set de date propriu mută conținutul vechi într-un folder de lângă el, așa că setul de date de deasupra are nevoie de loc pentru el.

### ZimaOS, CasaOS și alte gazde {#other-hosts}

- **ZimaOS și CasaOS:** adaugă [magazinul comunității](https://github.com/junkerderprovinz/zimaos-apps#install) și instalează BombVault din el. Rulează acolo ca `PLATFORM=generic`, cu Host Data pe `/DATA`. ZimaOS ține datele aplicațiilor în `/DATA/AppData/<app>`, pe care BombVault le găsește singur, pentru că `DATA_ROOT_SEGMENTS` potrivește `appdata` indiferent de litere mari sau mici.
- **MOS:** instalează BombVault din [MOS Hub](https://mos-official.net/apps.html?search=BombVault). Rulează acolo ca `PLATFORM=generic`, cu Host Data pe `/mnt/cache`.
- **Synology:** în Container Manager, creează un proiect din fișierul compose (DSM 7.2 sau mai nou).
- **Proxmox:** rulează-l într-o VM sau într-un container LXC care are Docker.

Fără comentarii, fișierul compose generic este scurt. Înlocuiește `./data` cu folderul gazdei care conține datele containerelor tale, setează `HOST_SOURCE_ROOT` la aceeași cale și completează `APP_KEY`:

```yaml
services:
  bombvault:
    image: junkerderprovinz/bombvault:latest
    restart: unless-stopped
    hostname: bombvault
    container_name: bombvault
    volumes:
      - ./config:/config
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/host/user
    environment:
      APP_KEY: ""
      PLATFORM: generic
      HOST_SOURCE_ROOT: /host/user
    ports:
      - "3443:3443"
```

Ca să salvezi și volumele Docker cu nume, montează un folder care conține atât datele tale, cât și rădăcina de date a Docker (`docker info -f '{{.DockerRootDir}}'`, de obicei `/var/lib/docker`), așa cum șablonul Unraid montează tot `/mnt`.

## Singura setare obligatorie {#the-one-required-setting}

Singura variabilă pe care trebuie să o setezi este `APP_KEY`, un secret hex de 32 de octeți (64 de caractere hex) folosit pentru a deriva parola depozitului restic.

Generează unul pe orice mașină:

```bash
openssl rand -hex 32
```

Lipește rezultatul în câmpul `APP_KEY` al șablonului (Unraid) sau în variabila de mediu `APP_KEY` din `docker-compose.yml` (gazdă Docker generică).

!!! danger "Nu-ți pierde APP_KEY"
    Pierderea `APP_KEY` face ca backupurile tale criptate să nu mai poată fi recuperate. Păstrează-l undeva în siguranță și separat de server. Odată ce BombVault rulează, setează o parolă de conectare și descarcă **kitul de recuperare** de la **Setări, Restaurare** (vezi [Off-site și recuperare](offsite-recovery.md#recovery-kit)). Conține cheia, toate destinațiile și comenzile restic.

Șablonul montează pentru tine și socket-ul Docker, flash-ul (`/boot`) și rădăcina **Host Data** (`/mnt`). Atât *sursele*, cât și *destinațiile* backupurilor se află sub Host Data. Pentru referința completă a variabilelor și configurarea off-site, vezi [Configurare](configuration.md).

## Prima rulare {#first-run}

![Pagina de bun venit a unei instalări noi, cu ce a găsit BombVault pe server.](assets/screenshots/getting-started/welcome-dark.png#only-dark)
![Pagina de bun venit a unei instalări noi, cu ce a găsit BombVault pe server.](assets/screenshots/getting-started/welcome-light.png#only-light)

1. Deschide interfața web la `https://<your-unraid-ip>:3443` (certificat auto-semnat implicit). O instalare nouă se deschide la **Bun venit în BombVault**, care enumeră ce a găsit BombVault pe server.
2. Alege **Să începem**. Prezentarea arată apoi cardul **Primii pași** cu trei pași:
    1. **Alege secțiunile**: pornește ce salvează BombVault (Containere, VM-uri, Flash, Foldere, Seturi de date ZFS, Auto-backup). **Alege** deschide lista elementelor unei secțiuni, unde **Pune programarea pe pauză** lasă unul deoparte. Secțiunile pe care le pornești aici rulează în fiecare noapte, lucru pe care îl schimbi la **Setări, Programare**.
    2. **Adaugă destinația**: un al doilea loc pentru backupuri. Fiecare secțiune are deja un folder pe acest server, `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}`, creat la primul backup. Cel mai sigur este o a doua destinație în afara locuinței, de exemplu un Storage Box sau Backblaze B2.
    3. **Prima copie de rezervă**: după aceea rulează singură în fiecare noapte. Cardul rămâne până se termină primul backup.
3. Deschide **Copii de rezervă** ca să vezi toate elementele într-o singură listă. **Copiază tot acum** pornește o rulare peste toate secțiunile pornite, iar **Copiază acum** pe pagina unui element salvează doar acel element.

![Cardul Primii pași cu cei trei pași, al doilea deschis.](assets/screenshots/getting-started/first-steps-dark.png#only-dark)
![Cardul Primii pași cu cei trei pași, al doilea deschis.](assets/screenshots/getting-started/first-steps-light.png#only-light)

Ai mai folosit BombVault? **Restaurează dintr-o copie de rezervă** de pe pagina de bun venit deschide pagina Restaurare, unde **Asistentul de urgență** readuce setările, containerele, VM-urile și folderele unui server pierdut. Vezi [Off-site și recuperare](offsite-recovery.md#emergency).

!!! tip "Opțional: alege o ordine a backupurilor"
    Dacă unele containere ar trebui să fie mereu salvate înaintea altora (de exemplu o bază de date înaintea aplicației care o folosește), deschide cardul **Ordine** de la **Setări, Programare** și trage-le în ordinea dorită. Rulările programate și backupul total o urmează apoi. Orice lași neordonat este salvat începând cu cel mai întârziat.

## Exemplu: primul tău backup de container, de la cap la coadă {#example-first-backup}

Aici parcurgi o stație Unraid nouă, cu câteva containere, printre care `plex`, până când primul backup este gata și știi unde să-l găsești. Durează cam zece minute, plus timpul primului backup.

1. **Instalează.** În fila **Apps** din Unraid, caută **BombVault** și apasă **Install**.
2. **Setează cheia.** Pe un calculator, rulează `openssl rand -hex 32` și lipește cele 64 de caractere în câmpul `APP_KEY` al șablonului. Păstrează acum o copie în managerul tău de parole, înainte de orice altceva. Apasă **Apply**.
3. **Deschide-l.** Mergi la `https://<your-unraid-ip>:3443` și acceptă certificatul auto-semnat. **Bun venit în BombVault** listează ce a găsit, de exemplu 12 containere, 2 VM-uri și unitatea flash.
4. **Să începem.** Prezentarea se deschide cu cardul **Primii pași**.
5. **Alege secțiunile.** **Containere** este pornit de la început. Pornește ce altceva vrei, de exemplu **Flash**. **Alege** deschide lista containerelor găsite, unde **Pune programarea pe pauză** lasă unul deoparte, de exemplu un container de test de care nu-ți pasă. Secțiunile pornite rulează în fiecare noapte în backupul total.
6. **Adaugă destinația.** Fiecare secțiune are deja propriul folder pe acest server, sub `/mnt/user/bombvault`, așa că primul backup are unde să meargă. Acest pas adaugă un al doilea loc, iar cel mai sigur este în afara locuinței. Îl poți face acum cu [exemplul S3](offsite-recovery.md#example-s3) sau te poți întoarce la el după primul backup. Cardul rămâne până atunci.
7. **Prima copie de rezervă.** Deschide **Copii de rezervă** și apasă **Copiază tot acum** sau așteaptă noaptea. Inelul din antetul de stare devine un inel de progres și numără elementele, de exemplu **Se face backup acum: 3 din 12**. Fiecare container este oprit cât îi sunt citite datele și pornit din nou imediat după. Un container ale cărui date se află pe ZFS este oprit doar cele câteva secunde ale unui instantaneu. Primul backup citește tot, deci durează cel mai mult. Cele de după stochează doar ce s-a schimbat.
8. **Verifică.** Când inelul spune **Totul este salvat**, deschide **Istoric** în prezentare: fiecare container are un rând verde în **Rulări**. Deschide **Copii de rezervă**, fă clic pe `plex` și uită-te la fila lui **Copii de rezervă**: primul instantaneu este acolo, cu **Local** ca loc care îl conține. Câteva minute mai târziu, fila lui **Verificare** arată verificarea de restaurare, care a restaurat un eșantion din backup și l-a citit înapoi.
9. **Protejează cheia.** Deschide **Setări, Securitate** și setează o parolă de conectare. Apoi deschide **Setări, Restaurare** și apasă **Descarcă kitul de recuperare** în **Pentru urgențe**. Păstrează fișierul în afara serverului, lângă copia lui `APP_KEY` de la pasul 2. Memento-ul **Salvează-ți kitul de recuperare** din prezentare dispare după ce apeși **L-am păstrat în siguranță**.

De acum containerele sunt salvate în fiecare noapte. Pași următori buni, în această ordine: o [destinație off-site](offsite-recovery.md#example-s3), ca un server mort să nu ia backupurile cu el, apoi [notificările](notifications.md), ca un eșec să ajungă la tine, apoi o privire la [Programări](schedules.md) dacă 03:00 nu ți se potrivește.

![Antetul de stare cât rulează primul backup, cu inelul de progres și stațiile lui.](assets/screenshots/getting-started/status-ring-dark.png#only-dark)
![Antetul de stare cât rulează primul backup, cu inelul de progres și stațiile lui.](assets/screenshots/getting-started/status-ring-light.png#only-light)

!!! note "Verificare integrare gazdă"
    Cardul **Integrare gazdă** din prezentare sondează fiecare montare și CLI (socket Docker, libvirt, restic, qemu-img, rclone) și raportează orice element lipsă, astfel încât să poți confirma că containerul este cablat corect înainte să te bazezi pe el. Așteaptă la **Personalizează** până îl adaugi.

## Să te orientezi {#navigation}

![Setările ca plăci, cu cardurile plăcii alese alături. Fiecare modificare se scrie pe loc.](assets/screenshots/getting-started/settings-rail-dark.png#only-dark)
![Setările ca plăci, cu cardurile plăcii alese alături. Fiecare modificare se scrie pe loc.](assets/screenshots/getting-started/settings-rail-light.png#only-light)

Bara laterală are cinci intrări: **Prezentare**, **Anomalii** (cât timp detectarea anomaliilor este pornită), **Copii de rezervă**, **Restaurare** și, sub **Deconectare**, **Setări**. Acțiunea principală a unei pagini plutește jos în dreapta, iar **Mai mult** le conține pe cele rare. Pe telefon bara conține Prezentare, Copii de rezervă, Restaurare și Setări, iar **Mai mult** are restul.

Paginile și setările folosesc toată lățimea ferestrei. O fereastră își ține butoanele în subsol, cu cel care merge înainte în dreapta. Pe telefon, un subsol cu mai mult de trei butoane păstrează ieșirea și acțiunea principală, iar restul le pune sub **Mai mult**.

![Pe telefon, bara de jos cu Mai mult deschis.](assets/screenshots/getting-started/phone-more-dark.png#only-dark)
![Pe telefon, bara de jos cu Mai mult deschis.](assets/screenshots/getting-started/phone-more-light.png#only-light)

- **Prezentare** arată dacă totul este salvat și ce are nevoie de tine. Vezi [Prezentare](overview.md).
- **Copii de rezervă** enumeră ca elemente fiecare container, VM, set de foldere, unitatea flash, fiecare set de date ZFS și auto-backupul. Plăcile de deasupra listei aleg un tip, iar **Filtre**, **Sortare** și **Caută** restrâng lista. Fiecare element are o pagină proprie cu trei file: **Copii de rezervă**, **Setări** și **Verificare**. Un buton înapoi, la stânga pictogramei elementului, duce la listă. Vezi [Backupuri și elemente](backups.md).
- **Restaurare** este o singură pagină pentru tot: alegi o secțiune, cauți un element și alegi o copie de rezervă. Asistentul de urgență și celelalte surse pornesc tot de aici. Vezi [Restaurare](restore.md).
- **Setările** sunt paisprezece plăci: General, Aspect, Programare, Destinații, Restaurare, Notificări, Verificare, Securitate, Instanțe, Împerechere, Integrări, Aplicații, Sistem și Info. Vezi [Setări](settings.md). Apasă `/` sau Ctrl+K ca să le cauți pe toate, și după numele pe care paginile le aveau în versiunile mai vechi. Trage plăcile în ordinea ta la **Aspect**. Unde o coloană de plăci de lângă pagină ar strânge pagina, ca într-o fereastră lată de aproximativ 1200 de pixeli, plăcile devin o bandă deasupra paginii, care derulează lateral. Un telefon arată aceeași bandă.
- **Destinații** și **Instanțe** sunt plăci de setări cu pagini proprii. **Arată și în bara laterală** pe placa lor le fixează în bara laterală.

Nimic nu are buton de salvare și nimic nu se desfășoară: lucrurile rare au un card al lor sau o fereastră, iar explicațiile stau în baloanele (i). După o actualizare se deschide singură o fereastră cu un card pentru fiecare lansare de la ultima ta versiune, iar punctele verzi marchează ce s-a schimbat. Un clic ascunde un punct. **Nou** din cardul Versiune de la **Setări, Info** numără ce nu ai văzut încă și deschide din nou acea fereastră, cu notele de lansare ale fiecărei lansări și schimbările ei cu **Mergi acolo**.

## Compilare din sursă {#build-from-source}

BombVault este un singur binar Go static care servește un API JSON și o interfață React încorporată. Compilează mai întâi interfața, apoi rulează binarul:

```bash
npm --prefix web ci
npm --prefix web run build     # writes web/dist, which the binary embeds
export APP_KEY=$(openssl rand -hex 32)
go test ./...                  # unit and integration tests, with a real restic round trip
golangci-lint run ./...
go run ./cmd/bombvault         # serves https://localhost:3443 with a self-signed certificate
```

Compilarea interfeței este necesară și pentru `go run`. Depozitul urmărește doar un marcaj gol sub `web/dist`, așa că fără `npm --prefix web run build` binarul nu încorporează nimic și răspunde `500 SPA index not found`, ceea ce este de așteptat. Docker, libvirt și Unraid nu pot fi testate în CI, așa că verifică montările, restic și legătura SSH pentru VM pe o gazdă reală cu cardul **Integrare gazdă** din prezentare (sau `GET /api/spike`) înainte să deschizi un pull request.

## Pașii următori {#next-steps}

- Adaugă una sau mai multe destinații în afara locuinței (fiecare secțiune poate copia în mai multe simultan) și salvează-ți kitul de recuperare, așa cum este descris la **[Off-site și recuperare](offsite-recovery.md)**. [Exemplul S3](offsite-recovery.md#example-s3) parcurge una.
- Asigură-te că un eșec ajunge la tine cu **[Notificările](notifications.md)**.
- Verifică ce rulează când la **[Programări](schedules.md)**.
- Pune toate serverele grupului tău pe telefon cu **[Aplicația Android](android.md)**.
- **[Exemplele lucrate](guides.md)** parcurg sarcinile obișnuite, iar **[Funcționalități](features.md)** enumeră tot ce face BombVault.
- Clonezi o configurație sau te muți pe o stație nouă? Mută-ți întreaga configurație cu cardul **Exportă / importă setările**. Vezi [Configurare](configuration.md#portable-settings-export-and-import).
- Te-ai blocat? Vezi **[Depanare](troubleshooting.md)**.
