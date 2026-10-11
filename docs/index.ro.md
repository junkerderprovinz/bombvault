# BombVault

**Datele tale Unraid, sigilate într-un seif. Depune un backup. Detonează o restaurare.**

BombVault este o aplicație web self-hosted, nativă pentru Unraid, pentru **backup și recuperare completă în caz de dezastru** pentru containerele tale Docker și mașinile virtuale KVM/libvirt. Rulează ca un singur container Docker multi-arch, îți oferă o interfață web cu temă închisă, deschisă sau cea a sistemului, și gestionează întregul ciclu de viață: backup, programare, verificare și restaurare.

Restaurările sunt automate. Containerele reapar în fila Docker din Unraid exact ca înainte, iar VM-urile sunt redefinite în VM Manager cu discurile și NVRAM-ul UEFI reatașate. Fără reinstalare manuală, fără reconfigurare, fără drame.

Bazat pe [restic](https://restic.net), așa că fiecare backup este deduplicat, incremental și mereu criptat.

!!! note "Păstrează-ți APP_KEY în siguranță"
    BombVault derivă parola depozitului restic dintr-un secret de 32 de octeți numit `APP_KEY`. Pierderea lui, împreună cu kitul de recuperare, face ca copiile de rezervă criptate să nu mai poată fi recuperate. Generează unul cu `openssl rand -hex 32` și păstrează-l undeva în siguranță. Vezi [Configurare](configuration.md).

## Ce protejează BombVault

Fiecare container, VM, set de foldere, unitatea flash, fiecare set de date ZFS și propriile setări ale BombVault sunt un **element** cu o pagină proprie. Elementele de același fel formează o **secțiune**, pe care o activezi la **Setări, General**.

| Secțiune | Ce se salvează |
|---|---|
| **Containere** | Directorul appdata plus definiția containerului (imagine, variabile de mediu, porturi, etichete, volume). |
| **VM-uri** | Imaginea (imaginile) de disc a VM-ului, definiția XML și NVRAM-ul UEFI, salvate prin SSH (fără montare libvirt). |
| **Flash** | Întregul flash USB (`/boot`): sistemul de operare, licența, configurația array-ului, partajările, rețeaua și configurația plugin-urilor. |
| **Auto-backup** | Propriul `/config` al BombVault: baza sa de date de setări, datele de autentificare ale destinațiilor sale și perechea de chei SSH pentru libvirt. |
| **Foldere** | Orice folder de pe server ca set de foldere, fiecare cu tipare de excludere opționale. |
| **Seturi de date ZFS** | Un set de date cu toate seturile de sub el, citit dintr-un instantaneu ZFS și salvat ca un folder. Vezi [Seturi de date ZFS](zfs-datasets.md). |

## Restaurarea este vedeta

După copierea datelor înapoi din instantaneul restic, BombVault reaplică definiția salvată a containerului prin API-ul Docker, astfel încât containerul reapare în fila Docker din Unraid ca și cum ar fi fost mereu acolo (aceeași imagine, aceleași setări, aceleași mapări de porturi). VM-urile își au XML-ul redefinit prin SSH și discurile și NVRAM-ul UEFI reatașate, chiar și după ce VM-ul a fost șters.

O singură fereastră restaurează orice fel de element. Implicit alege **Lângă original**, care aduce copia de rezervă înapoi ca o copie și lasă originalul neatins. **Înlocuiește originalul** face mai întâi o copie de rezervă a stării curente, așa că o restaurare poate fi anulată. Un folder sau un set de date ZFS nou sunt celelalte ținte.

Când un backup oprește containere dependente, ele revin în ordinea corectă: BombVault le repornește în ordinea `depends_on` din Compose și așteaptă ca fiecare să raporteze că este sănătos înainte de a le porni pe cele care depind de el, astfel încât nimic nu o ia înainte unei baze de date sau unui gateway care nu este încă activ. Vezi [Funcționalități](features.md).

## Cum funcționează

```
Browser --HTTPS--> BombVault container
                   |- Go binary: JSON API + embedded React UI
                   |- Background worker (per-section scheduler + job executor)
                   |
                   |- /var/run/docker.sock  -> Docker API (container stop/inspect/recreate)
                   |- qemu+ssh://host       -> libvirt / KVM on the HOST over SSH (no mount)
                   |- /mnt/ -> /host/user   -> appdata, VM disks + restic repos (read/write)
                   |- /boot/ -> /host/boot  -> Unraid flash backup (whole USB)
                   |- /config               -> BombVault's own settings + credentials (self-backup)
                   '- <repo path>           -> restic repository (local or remote: rclone/s3/rest/sftp)
```

BombVault folosește socketul Docker pentru a opri containerele înainte de un backup și pentru a le recrea după o restaurare. Pentru VM-uri rulează `virsh` pe gazdă prin SSH (`qemu+ssh://`) ca să oprească un VM în mod controlat sau să facă un instantaneu live. Nu montează niciodată o cale libvirt, așa că nu poate încurca VM Manager-ul de pe gazdă.

BombVault este stratul de orchestrare și interfață, nu motorul de stocare. Toată mișcarea reală a datelor trece prin restic.

## Pornire rapidă

Nou pe aici? Mergi la **[Primii pași](getting-started.md)** pentru a instala BombVault pe Unraid prin Community Applications și a rula primul tău backup. Apoi explorează toate **[Funcționalitățile](features.md)**, ajustează-ți **[Configurarea](configuration.md)** și configurează **[Off-site și recuperare](offsite-recovery.md)**.

Fiecare secțiune poate copia simultan în mai multe destinații, un BombVault împerecheat poate păstra copiile tale externe pe un server care refuză ștergerile, iar **Asistentul de urgență** readuce un server pierdut din kitul lui de recuperare, prin grupul tău sau cu cheia tastată. Ca să-ți muți configurația pe o stație nouă, folosește cardul **Exportă / importă setările**. Vezi [Off-site și recuperare](offsite-recovery.md) și [Configurare](configuration.md#portable-settings-export-and-import).

**[Aplicația Android](android.md)** pune toate serverele grupului tău pe telefon, cu jurnalul de activitate al tuturor pe un singur ecran.

## Mulțumiri {#credits}

- **[VolumeVault](https://github.com/Darkdragon14/VolumeVault)** de [@Darkdragon14](https://github.com/Darkdragon14) (Apache-2.0) i-a dat lui BombVault ideea de pornire: backup dintr-un clic și reinstalarea automată a containerelor Docker. BombVault este o implementare separată, pe Go și restic, care duce ideea mai departe la VM-uri, la flash și nu numai.
- **[restic](https://restic.net/)** este motorul de backup rapid, sigur și cu deduplicare pe care îl folosește BombVault.
- **[rclone](https://rclone.org/)** oferă backendurile cloud.
- Majoritatea glifelor de pe butoane provin din setul gratuit Core Solid de la **[Streamline](https://streamlinehq.com)** ([CC BY 4.0](https://creativecommons.org/licenses/by/4.0/), [sursă](https://github.com/webalys-hq/streamline-vectors)). Celelalte provin din Font Awesome Free, Material Design Icons, Simple Icons și Tabler Icons sau au fost desenate pentru proiect.

## Licență {#license}

Copyright (C) 2026 Junker der Provinz. BombVault este software liber sub **GNU Affero General Public License v3.0** ([LICENSE](https://github.com/junkerderprovinz/bombvault/blob/main/LICENSE)). Îl poți rula, studia, distribui și modifica. Dacă îl distribui sau rulezi o versiune modificată ca serviciu de rețea, trebuie să publici codul sursă sub aceeași licență și să păstrezi notificările existente de copyright și de atribuire.

Numele și identitatea vizuală nu sunt licențiate. AGPL acoperă doar codul sursă: "BombVault", logoul și identitatea sa vizuală rămân rezervate, așa că un fork trebuie să folosească un nume și o identitate vizuală proprii și nu are voie să se prezinte drept BombVault.

## Linkuri

- **Cod sursă:** [github.com/junkerderprovinz/bombvault](https://github.com/junkerderprovinz/bombvault)
- **Fir de suport Unraid:** [forums.unraid.net](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/)
- **Probleme:** [github.com/junkerderprovinz/bombvault/issues](https://github.com/junkerderprovinz/bombvault/issues)

!!! warning "Control al gazdei echivalent cu root"
    Prin socket-ul Docker, BombVault poate opri, elimina și recrea containere și poate citi/scrie appdata, iar pentru backupul VM se autentifică pe gazdă prin SSH pentru a rula `virsh`. Oricine poate ajunge la interfața sa web are efectiv root pe gazdă. Rulează BombVault doar într-o rețea de încredere, neexpusă, și setează o parolă de conectare la **Setări, Securitate**. Kitul de recuperare, depunerile de chei și ștergerea copiilor de rezervă de la o destinație au nevoie de una. Vezi [Configurare](configuration.md#security-model) pentru modelul complet de securitate.
