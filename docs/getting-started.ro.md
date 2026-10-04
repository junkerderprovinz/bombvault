# Primii pași

Această pagină te conduce de la o stație Unraid nouă până la primul tău backup.

## Cerințe

| Cerință | Note |
|---|---|
| **Unraid 6.12+** | Versiunile mai vechi nu sunt testate. Unraid este ținta principală, dar BombVault rulează și pe o gazdă Docker obișnuită și pe TrueNAS Scale (vezi [Gazdă Docker generică](#generic-docker-host)). |
| **Locația depozitului restic** | O cale locală (recomandat: array-ul sau cache-ul tău), SMB, NFS sau orice backend rclone. |
| **Socket Docker** | Montat automat de șablon (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Montat integral de șablon, automat (`/boot` la `/host/boot`). Alimentează backupul flash și permite ca un container restaurat să reapară ca o aplicație Unraid normală, editabilă. |
| **VM-uri KVM** (opțional) | Backupul VM comunică cu libvirt prin SSH, fără montare libvirt. Configurează-l în Setări (vezi [Configurare](configuration.md)). |
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

Nu ești pe Unraid? BombVault rulează și ca un container obișnuit pe orice gazdă Docker (tot pe asta se sprijină și suportul pentru containere pe TrueNAS Scale, până la o intrare proprie în catalogul de aplicații de acolo).

1. Ia din depozit fișierul [`deploy/docker-compose.generic.yml`](https://github.com/junkerderprovinz/bombvault/blob/main/deploy/docker-compose.generic.yml), gata de editat.
2. Setează `APP_KEY` (vezi mai jos) și îndreaptă volumul Host Data către rădăcina ta reală de date: comentariile din fișier le parcurg pe amândouă.
3. `docker compose up -d`, apoi deschide `https://<ip-gazdă>:3443/`.

Ce este diferit față de Unraid:

- **Nu există domeniu flash/USB.** Nu există un stick de pornire de capturat sau de restaurat, așa că domeniul Flash din setări nu are ce face aici. În schimb, domeniul Foldere oferă sugestia dintr-un clic **Adaugă presetare: configurația sistemului gazdă** (un set inițial de fișiere `/etc` pe care îl revizuiești și îl modifici înainte de salvare), ca echivalent generic practic.
- **Nu există notificări native Unraid.** Canalele proprii de notificare ale BombVault (webhook, alerte de eșec în afara sediului și așa mai departe) funcționează ca de obicei; se omite doar trimiterea specifică sistemului de notificări al Unraid, întrucât aici nu există un asemenea sistem.
- **Copierea mașinilor virtuale este opțională și are nevoie de o gazdă libvirtd separată, accesibilă prin SSH.** Vezi blocul comentat din fișierul compose. O gazdă Docker generică nu vine cu niciun administrator de mașini virtuale.
- **Nu există widget pentru panou.** BombVault Widget este un plugin Unraid, așa că și acest pas este omis.
- **Găsirea datelor unui container.** Fără convenția `appdata` a Unraid, folderul de date al unui container este găsit pornind de la segmentele din `DATA_ROOT_SEGMENTS`, volumele Docker cu nume, directorul de lucru al unui proiect Compose și eticheta `bombvault.data` (vezi [Detectarea surselor de copie de rezervă](configuration.md#backup-source-detection)). Volumele cu nume și presetarea `/etc` ajung doar la căi din interiorul montării Host Data, așa că îndreaptă Host Data către un director părinte comun care acoperă și rădăcina de date a Docker.
- **`PLATFORM`.** Seteaz-o la `generic` sau `truenas`. Dacă nu este setată, BombVault recunoaște Unraid după propriul marcaj de pe montarea flash și tratează orice altceva ca generic, iar pașii specifici Unraid sunt omiși în loc să fie încercați și să eșueze.

**TrueNAS Scale** urmează aceeași cale cu compose; o intrare de catalog este pregătită în depozit, dar încă netrimisă. Acolo backupul VM are nevoie de `LIBVIRT_URI`, pentru că libvirtd-ul din TrueNAS ascultă pe un socket propriu (`/run/truenas_libvirt/libvirt-sock`) pe care cele trei variabile `LIBVIRT_*` nu îl pot exprima (vezi [Configurare](configuration.md)). Cât de departe s-a verificat: backupul de zvol a fost rulat pe o stație TrueNAS Scale reală, pe un zvol atașat unui VM pornit, iar `zfs snapshot`, `zfs send`, restic și `zfs receive` l-au dus și l-au adus înapoi identic, octet cu octet. O restaurare completă condusă chiar de BombVault nu a fost încă rulată pe hardware TrueNAS, iar acel zvol era sparse, așa că debitul la mulți gigaocteți nu este testat. Testează acolo o restaurare înainte să te bazezi pe ea.

## Singura setare obligatorie

Singura variabilă pe care trebuie să o setezi este `APP_KEY`, un secret hex de 32 de octeți (64 de caractere hex) folosit pentru a deriva parola depozitului restic.

Generează unul pe orice mașină:

```bash
openssl rand -hex 32
```

Lipește rezultatul în câmpul `APP_KEY` al șablonului (Unraid) sau în variabila de mediu `APP_KEY` din `docker-compose.yml` (gazdă Docker generică).

!!! danger "Nu-ți pierde APP_KEY"
    Pierderea `APP_KEY` face ca backupurile tale criptate să nu mai poată fi recuperate. Păstrează-l undeva în siguranță și separat de server. Odată ce BombVault rulează, folosește **kitul de recuperare a cheii de criptare** cu un singur clic (vezi [Off-site și recuperare](offsite-recovery.md)) pentru a salva pachetul complet de recuperare.

Șablonul montează pentru tine și socket-ul Docker, flash-ul (`/boot`) și rădăcina **Host Data** (`/mnt`). Atât *sursele* cât și *destinațiile* backupurilor se află sub Host Data. Pentru referința completă a variabilelor și configurarea off-site, vezi [Configurare](configuration.md).

## Prima rulare

![Tabloul după prima copie: ce e protejat, ce rulează în continuare și un jurnal viu.](assets/screenshots/dashboard.png)

*Tabloul după prima copie: ce e protejat, ce rulează în continuare și un jurnal viu.*

1. Deschide interfața web la `https://<your-unraid-ip>:3443` (certificat auto-semnat implicit).
2. În **Setări**, activează domeniile de backup dorite (Containere, VM-uri, Flash, Auto-backup, Foldere, Seturi de date ZFS) și alege o culoare de accent.
3. În fila **Containere**, alege un container și apasă **Copiază acum** pentru a-ți crea primul punct de restaurare. Căile depozitelor implicite sunt `/mnt/user/bombvault/{container,vms,flash,config,files,zfs}` și sunt create la primul backup.
4. Configurează programarea din **Setări, Programări**. Există un *Include toate în programare* cu un singur clic pentru containere și VM-uri.

!!! tip "Opțional: alege o ordine de backup"
    Dacă unele containere ar trebui să fie mereu salvate înaintea altora (de exemplu o bază de date înaintea aplicației care o folosește), deschide panoul **Ordinea backupurilor** din pagina Containere și trage-le în ordinea dorită. Rulările programate și cele cu selecție multiplă o urmează apoi; orice lași neordonat este salvat în ordinea celor mai restante mai întâi, ca înainte.

!!! note "Verificare integrare gazdă"
    Deschide `/spike` în interfața web după ce containerul pornește. Sondează fiecare montare și CLI (socket Docker, libvirt, restic, qemu-img, rclone) și raportează orice element lipsă, astfel încât să poți confirma că containerul este cablat corect înainte să te bazezi pe el.

## Simplu vs Avansat

![Setările nu au buton de Salvare: fiecare modificare se scrie pe loc.](assets/screenshots/settings.png)

*Setările nu au buton de Salvare: fiecare modificare se scrie pe loc.*

Implicit, interfața arată doar elementele esențiale (backup, restaurare, programare). Folosește comutatorul **Vizualizare simplă / Vizualizare avansată** din bara laterală pentru a dezvălui controalele pentru experți: retenție, copie off-site, hook-uri pre/post, restaurare la nivel de fișier, notificări, metrici Prometheus și instrumentele de integritate/mentenanță. Este o preferință per browser și oprită implicit, așa că noii veniți primesc o interfață curată, iar utilizatorii avansați primesc totul.

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

Compilarea interfeței este necesară și pentru `go run`. Depozitul urmărește doar un marcaj gol sub `web/dist`, așa că fără `npm --prefix web run build` binarul nu încorporează nimic și răspunde `500 SPA index not found`, ceea ce este de așteptat. Docker, libvirt și Unraid nu pot fi testate în CI, așa că verifică montările, restic și legătura SSH pentru VM pe o gazdă reală cu verificarea integrării gazdei (`/spike`) înainte să deschizi un pull request.

## Pașii următori

- Răsfoiește toate **[Funcționalitățile](features.md)**.
- Pune toate serverele grupului tău pe telefon cu **[Aplicația Android](android.md)**.
- Adaugă una sau mai multe replici **[Off-site și recuperare](offsite-recovery.md)** (fiecare domeniu poate trimite către mai multe destinații simultan) și salvează-ți kitul de recuperare.
- Clonezi o configurație sau te muți pe o stație nouă? Mută-ți întreaga configurație cu cardul **Exportă / importă setările**. Vezi [Configurare](configuration.md#portable-settings-export-and-import).
- Te-ai blocat? Vezi **[Depanare](troubleshooting.md)**.
