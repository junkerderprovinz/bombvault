# Primii pași

Această pagină te conduce de la o stație Unraid nouă până la primul tău backup.

## Cerințe

| Cerință | Note |
|---|---|
| **Unraid 6.12+** | Versiunile mai vechi nu sunt testate. Unraid este ținta principală, dar BombVault rulează și pe o gazdă Docker obișnuită și pe TrueNAS Scale (vezi [Gazdă Docker generică](#generic-docker-host) și [TrueNAS SCALE](#truenas)). |
| **Locația depozitului restic** | O cale locală (recomandat: array-ul sau cache-ul tău), SMB, NFS sau orice backend rclone. |
| **Socket Docker** | Montat automat de șablon (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Montat integral de șablon, automat (`/boot` la `/host/boot`). Alimentează backupul flash și permite ca un container restaurat să reapară ca o aplicație Unraid normală, editabilă. |
| **VM-uri KVM** (opțional) | Backupul VM comunică cu libvirt prin SSH, fără montare libvirt. Configurează-l la **Setări, Integrări, SSH al gazdei** (vezi [Configurare](configuration.md)). |
| **Seturi de date ZFS** (opțional) | Aceeași legătură SSH ca la backupul VM, `zfs` pe gazdă și Host Data mapat ca `/mnt` cu modul de acces Read/Write - Slave, valoarea implicită a șablonului. Vezi [Seturi de date ZFS](zfs-datasets.md). |
| **Aplicația Android** (facultativ) | Android 10 sau mai nou, împerecheată cu servere la versiunea 9.7.0 sau mai nouă. Vezi [Aplicația Android](android.md). |

## Instalare pe Unraid

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
2. Setează `APP_KEY` (vezi mai jos) și îndreaptă volumul Host Data către rădăcina ta reală de date: comentariile din fișier le parcurg pe amândouă.
3. `docker compose up -d`, apoi deschide `https://<ip-gazdă>:3443/`.

Ce este diferit față de Unraid:

- **Nu există secțiunea Flash.** Nu există un stick de pornire de capturat sau de restaurat, așa că secțiunea Flash nu are ce face aici. În schimb, când Host Data ajunge la `/etc` al gazdei, **Adaugă** de pe pagina Copii de rezervă oferă **Configurația sistemului** ca sugestie de folder: un set inițial de foldere `/etc` pe care îl revizuiești înainte de salvare.
- **Nu există notificări native Unraid.** Canalele proprii de notificare ale BombVault (webhook, alerte de eșec în afara sediului și așa mai departe) funcționează ca de obicei; se omite doar trimiterea specifică sistemului de notificări al Unraid, întrucât aici nu există un asemenea sistem.
- **Copierea mașinilor virtuale este opțională și are nevoie de o gazdă libvirtd separată, accesibilă prin SSH.** Vezi blocul comentat din fișierul compose. O gazdă Docker generică nu vine cu niciun administrator de mașini virtuale.
- **Nu există widget pentru panou.** BombVault Widget este un plugin Unraid, așa că și acest pas este omis.
- **Găsirea datelor unui container.** Fără convenția `appdata` a Unraid, folderul de date al unui container este găsit pornind de la segmentele din `DATA_ROOT_SEGMENTS`, volumele Docker cu nume, directorul de lucru al unui proiect Compose și eticheta `bombvault.data` (vezi [Detectarea surselor de copie de rezervă](configuration.md#backup-source-detection)). Volumele cu nume și presetarea `/etc` ajung doar la căi din interiorul montării Host Data, așa că îndreaptă Host Data către un director părinte comun care acoperă și rădăcina de date a Docker.
- **`PLATFORM`.** Seteaz-o la `generic` sau `truenas`. Dacă nu este setată, BombVault recunoaște Unraid după propriul marcaj de pe montarea flash și tratează orice altceva ca generic, iar pașii specifici Unraid sunt omiși în loc să fie încercați și să eșueze.

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
      # Pentru backupurile VM:
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
- **Datele dintr-un ixVolume nu intră în copie.** O aplicație al cărei spațiu de stocare este un ixVolume își ține datele sub `/mnt/.ix-apps`, în afara Host Data. BombVault salvează doar definiția aplicației, iar rularea numește folderele pe care le-a lăsat deoparte. Pentru fiecare aplicație ale cărei date vrei să le copiezi, alege stocare **Host Path** pe un set de date de sub Host Data.
- **Backupurile VM și ale seturilor de date ZFS** ajung la TrueNAS prin SSH ca root. Pornește serviciul SSH la **System, Services** și adaugă cheia publică din **Setări, Integrări, SSH al gazdei** la utilizatorul root, la **Credentials, Users**. Backupurile VM au nevoie și de `LIBVIRT_URI` ca mai sus, pentru că libvirtd-ul din TrueNAS ascultă pe un socket propriu. Dacă nu este setat nici `LIBVIRT_URI`, nici `LIBVIRT_HOST`, BombVault folosește ca gazdă gateway-ul implicit al aplicației, pentru că `host.docker.internal` nu se rezolvă într-o aplicație TrueNAS. [Ghidul de backup VM](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md#truenas-scale) acoperă detaliile, inclusiv VM-urile pe zvol-uri.
- **O replică ZFS folosește altă cheie.** O replică pe un al doilea server ZFS se conectează cu cheia pe care o arată dialogul replicii, nu cu cheia SSH a gazdei. Adaug-o pe aceea pe serverul care primește.
- **O aplicație ștearsă revine în doi pași.** Reinstaleaz-o mai întâi în TrueNAS cu același nume, ceea ce îi creează din nou rețeaua, apoi restaurează-i datele în BombVault. Până atunci, verificarea înainte de restaurare raportează rețeaua lipsă, iar restaurarea se oprește înainte să schimbe ceva.
- **O restaurare Lângă original** creează un container Docker obișnuit. Rulează, dar ecranul Apps din TrueNAS nu îl afișează.
- **Dumpurile bazelor de date** funcționează pe montări de seturi de date. Un import într-un folder de date care este un set de date propriu mută conținutul vechi într-un folder de lângă el, așa că setul de date de deasupra are nevoie de loc pentru el.

## Singura setare obligatorie

Singura variabilă pe care trebuie să o setezi este `APP_KEY`, un secret hex de 32 de octeți (64 de caractere hex) folosit pentru a deriva parola depozitului restic.

Generează unul pe orice mașină:

```bash
openssl rand -hex 32
```

Lipește rezultatul în câmpul `APP_KEY` al șablonului (Unraid) sau în variabila de mediu `APP_KEY` din `docker-compose.yml` (gazdă Docker generică).

!!! danger "Nu-ți pierde APP_KEY"
    Pierderea `APP_KEY` face ca copiile tale de rezervă criptate să nu mai poată fi recuperate. Păstrează-l undeva în siguranță și separat de server. Odată ce BombVault rulează, setează o parolă de conectare și descarcă **kitul de recuperare** de la **Setări, Restaurare** (vezi [Off-site și recuperare](offsite-recovery.md#recovery-kit)). Conține cheia, toate destinațiile și comenzile restic.

Șablonul montează pentru tine și socket-ul Docker, flash-ul (`/boot`) și rădăcina **Host Data** (`/mnt`). Atât *sursele* cât și *destinațiile* backupurilor se află sub Host Data. Pentru referința completă a variabilelor și configurarea off-site, vezi [Configurare](configuration.md).

## Prima rulare

![Prezentarea după prima copie de rezervă: ce necesită atenția ta, protecția fiecărei secțiuni, destinațiile și istoricul.](assets/screenshots/dashboard.png)

*Prezentarea după prima copie de rezervă: ce necesită atenția ta, protecția fiecărei secțiuni, destinațiile și istoricul.*

1. Deschide interfața web la `https://<your-unraid-ip>:3443` (certificat auto-semnat implicit). O instalare nouă se deschide la **Bun venit în BombVault**, care enumeră ce a găsit BombVault pe server.
2. Alege **Să începem**. Prezentarea arată apoi cardul **Primii pași** cu trei pași:
    1. **Alege secțiunile**: pornește ce salvează BombVault (Containere, VM-uri, Flash, Foldere, Seturi de date ZFS, Auto-backup). **Alege** deschide lista elementelor unei secțiuni, unde **Pune programarea pe pauză** lasă unul deoparte. Secțiunile pe care le pornești aici rulează în fiecare noapte, lucru pe care îl schimbi la **Setări, Programare**.
    2. **Adaugă destinație**: locul unde ajung copiile de rezervă. Un folder local ajunge pentru început. Cel mai sigur este o a doua destinație în afara locuinței, de exemplu un Storage Box sau Backblaze B2. Depozitele locale folosesc implicit `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` și sunt create la prima copie de rezervă.
    3. **Prima copie de rezervă**: după aceea rulează singură în fiecare noapte.
3. Deschide **Copii de rezervă** ca să vezi toate elementele într-o singură listă. **Copiază tot acum** pornește o rulare peste toate secțiunile pornite, iar **Copiază acum** pe pagina unui element salvează doar acel element.

Ai mai folosit BombVault? **Restaurează dintr-o copie de rezervă** de pe pagina de bun venit deschide pagina Restaurare, unde **Asistentul de urgență** readuce setările, containerele, VM-urile și folderele unui server pierdut. Vezi [Off-site și recuperare](offsite-recovery.md#emergency).

!!! tip "Opțional: alege o ordine a copiilor de rezervă"
    Dacă unele containere ar trebui să fie mereu salvate înaintea altora (de exemplu o bază de date înaintea aplicației care o folosește), deschide cardul **Ordine** de la **Setări, Programare** și trage-le în ordinea dorită. Rulările programate și copia de rezervă totală o urmează apoi; orice lași neordonat este salvat în ordinea celor mai restante mai întâi.

!!! note "Verificare integrare gazdă"
    Cardul **Integrare gazdă** de pe prezentare sondează fiecare montare și CLI (socket Docker, libvirt, restic, qemu-img, rclone) și raportează orice element lipsă, astfel încât să poți confirma că containerul este cablat corect înainte să te bazezi pe el. Așteaptă la **Personalizează** până îl adaugi.

## Să te orientezi {#navigation}

![Setările ca plăci, cu cardurile plăcii alese alături. Fiecare modificare se scrie pe loc.](assets/screenshots/settings.png)

*Setările ca plăci, cu cardurile plăcii alese alături. Fiecare modificare se scrie pe loc.*

Bara laterală are cinci intrări: **Prezentare**, **Anomalii** (cât timp detectarea anomaliilor este pornită), **Copii de rezervă**, **Restaurare** și, sub **Deconectare**, **Setări**. Acțiunea principală a unei pagini plutește jos în dreapta, iar **Mai mult** le conține pe cele rare. Pe telefon bara conține Prezentare, Copii de rezervă, Restaurare și Setări, iar **Mai mult** are restul.

Paginile și setările folosesc toată lățimea ferestrei. O fereastră își ține butoanele în subsol, cu cel care merge înainte în dreapta. Pe telefon, un subsol cu mai mult de trei butoane păstrează ieșirea și acțiunea principală, iar restul le pune sub **Mai mult**.

- **Copii de rezervă** enumeră ca elemente fiecare container, VM, set de foldere, unitatea flash, fiecare set de date ZFS și Auto-backup. Plăcile de deasupra listei aleg un fel, iar **Filtre**, **Sortare** și **Caută** restrâng lista. Fiecare element are o pagină proprie cu trei file: **Copii de rezervă**, **Setări** și **Verificare**. Un buton Înapoi, la stânga pictogramei elementului, duce la listă.
- **Restaurare** este o singură pagină pentru tot: alegi o secțiune, cauți un element și alegi o copie de rezervă. Asistentul de urgență și celelalte surse pornesc tot de aici.
- **Setările** sunt paisprezece plăci: General, Aspect, Programare, Destinații, Restaurare, Notificări, Verificare, Securitate, Instanțe, Împerechere, Integrări, Aplicații, Sistem și Info. Apasă `/` sau Ctrl+K ca să le cauți pe toate, și după numele pe care paginile le aveau în versiunile mai vechi. Trage plăcile în ordinea ta la **Aspect**. Unde o coloană de plăci de lângă pagină ar strânge pagina, ca într-o fereastră lată de aproximativ 1200 de pixeli, plăcile devin o bandă deasupra paginii, care derulează lateral. Un telefon arată aceeași bandă.
- **Destinații** și **Instanțe** sunt plăci de setări cu pagini proprii. **Arată și în bara laterală** pe placa lor le fixează în bara laterală.

Nimic nu are buton de salvare și nimic nu se desfășoară: lucrurile rare au un card al lor sau o fereastră, iar explicațiile stau în baloanele (i). După o actualizare se deschide singură o fereastră cu un card pentru fiecare lansare de la ultima ta versiune, iar punctele verzi marchează ce s-a schimbat. Un clic ascunde un punct. **Noutăți** din cardul Versiune de la **Setări, Info** numără ce nu ai văzut încă și deschide din nou acea fereastră, cu notele de lansare ale fiecărei lansări și schimbările ei cu **Mergi acolo**.

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

## Pașii următori

- Răsfoiește toate **[Funcționalitățile](features.md)**.
- Pune toate serverele grupului tău pe telefon cu **[Aplicația Android](android.md)**.
- Adaugă una sau mai multe destinații în afara locuinței (fiecare secțiune poate copia în mai multe simultan) și salvează-ți kitul de recuperare, așa cum este descris la **[Off-site și recuperare](offsite-recovery.md)**.
- Clonezi o configurație sau te muți pe o stație nouă? Mută-ți întreaga configurație cu cardul **Exportă / importă setările**. Vezi [Configurare](configuration.md#portable-settings-export-and-import).
- Te-ai blocat? Vezi **[Depanare](troubleshooting.md)**.
