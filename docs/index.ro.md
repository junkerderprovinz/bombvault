# BombVault

**Datele tale Unraid, sigilate într-un seif. Depune un backup. Detonează o restaurare.**

BombVault este o aplicație web self-hosted, nativă pentru Unraid, pentru **backup și recuperare completă în caz de dezastru** pentru containerele tale Docker și mașinile virtuale KVM/libvirt. Rulează ca un singur container Docker multi-arch, îți oferă o interfață web modernă în temă întunecată și gestionează întregul ciclu de viață: backup, programare, verificare și restaurare.

Restaurările sunt automate. Containerele reapar în fila Docker din Unraid exact ca înainte, iar VM-urile sunt redefinite în VM Manager cu discurile și NVRAM-ul UEFI reatașate. Fără reinstalare manuală, fără reconfigurare, fără drame.

Bazat pe [restic](https://restic.net), așa că fiecare backup este deduplicat, incremental și mereu criptat.

!!! note "Păstrează-ți APP_KEY în siguranță"
    BombVault derivă parola depozitului restic dintr-un secret de 32 de octeți numit `APP_KEY`. Pierderea lui face ca backupurile criptate să nu mai poată fi recuperate. Generează unul cu `openssl rand -hex 32` și păstrează-l undeva în siguranță. Vezi [Configurare](configuration.md).

## Ce protejează BombVault

| Domeniu | Ce se salvează |
|---|---|
| **Containere Docker** | Directorul appdata plus definiția containerului (imagine, variabile de mediu, porturi, etichete, volume). |
| **VM-uri KVM / libvirt** | Imaginea (imaginile) de disc a VM-ului, definiția XML și NVRAM-ul UEFI, salvate prin SSH (fără montare libvirt). |
| **Flash Unraid** | Întregul flash USB (`/boot`): sistemul de operare, licența, configurația array-ului, partajările, rețeaua și configurația plugin-urilor. |
| **Configurația aplicației** | Propriul `/config` al BombVault: baza sa de date de setări, credențialele off-site și perechea de chei SSH pentru libvirt. |
| **Fișiere și foldere** | **Seturi de fișiere** denumite, orice folder de pe server, fiecare cu tipare de excludere opționale per set. |

## Restaurarea este vedeta

După copierea datelor înapoi din instantaneul restic, BombVault reaplică definiția salvată a containerului prin API-ul Docker, astfel încât containerul reapare în fila Docker din Unraid ca și cum ar fi fost mereu acolo (aceeași imagine, aceleași setări, aceleași mapări de porturi). VM-urile își au XML-ul redefinit prin SSH și discurile și NVRAM-ul UEFI reatașate, chiar și după ce VM-ul a fost șters.

Când un backup oprește containere dependente, ele revin în ordinea corectă: BombVault le repornește în ordinea `depends_on` din Compose și așteaptă ca fiecare să raporteze că este sănătos înainte de a le porni pe cele care depind de el, astfel încât nimic nu o ia înainte unei baze de date sau unui gateway care nu este încă activ. Vezi [Funcționalități](features.md).

## Cum funcționează

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-domain scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault este stratul de orchestrare și interfață, nu motorul de stocare. Toată mișcarea reală a datelor trece prin restic.

## Pornire rapidă

Nou pe aici? Mergi la **[Primii pași](getting-started.md)** pentru a instala BombVault pe Unraid prin Community Applications și a rula primul tău backup. Apoi explorează toate **[Funcționalitățile](features.md)**, ajustează-ți **[Configurarea](configuration.md)** și configurează **[Off-site și recuperare](offsite-recovery.md)**.

Off-site poate distribui către mai multe ținte per domeniu simultan, un **panou de recepție** doar în citire monitorizează aceste copii pe stația care le primește, iar întreaga ta configurație o poți muta pe o stație nouă cu cardul **Export și import setări**. Vezi [Off-site și recuperare](offsite-recovery.md) și [Configurare](configuration.md#portable-settings-export-and-import).

## Linkuri

- **Cod sursă:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Fir de suport Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Probleme:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Control al gazdei echivalent cu root"
    Prin socket-ul Docker, BombVault poate opri, elimina și recrea containere și poate citi/scrie appdata, iar pentru backupul VM se autentifică pe gazdă prin SSH pentru a rula `virsh`. Oricine poate ajunge la interfața sa web are efectiv root pe gazdă. Rulează BombVault doar într-o rețea de încredere, neexpusă, și activează bariera opțională de parolă (Setări, Securitate) odată ce sunt folosite backupuri off-site sau imuabile. Vezi [Configurare](configuration.md) pentru modelul complet de securitate.
