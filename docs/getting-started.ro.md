# Primii pași

Această pagină te conduce de la o stație Unraid nouă până la primul tău backup.

## Cerințe

| Cerință | Note |
|---|---|
| **Unraid 6.12+** | Versiunile mai vechi nu sunt testate. |
| **Locația depozitului restic** | O cale locală (recomandat: array-ul sau cache-ul tău), SMB, NFS sau orice backend rclone. |
| **Socket Docker** | Montat automat de șablon (`/var/run/docker.sock`). |
| **Flash Unraid** (`/boot`) | Montat integral de șablon, automat (`/boot` la `/host/boot`). Alimentează backupul flash și permite ca un container restaurat să reapară ca o aplicație Unraid normală, editabilă. |
| **VM-uri KVM** (opțional) | Backupul VM comunică cu libvirt prin SSH, fără montare libvirt. Configurează-l în Setări (vezi [Configurare](configuration.md)). |

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

## Singura setare obligatorie

Singura variabilă pe care trebuie să o setezi este `APP_KEY`, un secret hex de 32 de octeți (64 de caractere hex) folosit pentru a deriva parola depozitului restic.

Generează unul pe orice mașină:

```bash
openssl rand -hex 32
```

Lipește rezultatul în câmpul `APP_KEY` al șablonului.

!!! danger "Nu-ți pierde APP_KEY"
    Pierderea `APP_KEY` face ca backupurile tale criptate să nu mai poată fi recuperate. Păstrează-l undeva în siguranță și separat de server. Odată ce BombVault rulează, folosește **kitul de recuperare a cheii de criptare** cu un singur clic (vezi [Off-site și recuperare](offsite-recovery.md)) pentru a salva pachetul complet de recuperare.

Șablonul montează pentru tine și socket-ul Docker, flash-ul (`/boot`) și rădăcina **Host Data** (`/mnt`). Atât *sursele* cât și *destinațiile* backupurilor se află sub Host Data. Pentru referința completă a variabilelor și configurarea off-site, vezi [Configurare](configuration.md).

## Prima rulare

1. Deschide interfața web la `https://<your-unraid-ip>:3443` (certificat auto-semnat implicit).
2. În **Setări**, activează domeniile de backup dorite (Containere, VM-uri, Flash, Config, Fișiere) și alege o culoare de accent.
3. În fila **Containere**, alege un container și apasă **Back up** pentru a-ți crea primul punct de restaurare. Căile depozitelor implicite sunt `/mnt/user/bombvault/{container,vms,flash,config,files}` și sunt create la primul backup.
4. Configurează programarea din **Setări, Programări**. Există un *include all in schedule* cu un singur clic pentru containere și VM-uri.

!!! tip "Opțional: alege o ordine de backup"
    Dacă unele containere ar trebui să fie mereu salvate înaintea altora (de exemplu o bază de date înaintea aplicației care o folosește), deschide panoul **backup-order** din pagina Containere și trage-le în ordinea dorită. Rulările programate și cele cu selecție multiplă o urmează apoi; orice lași neordonat este salvat în ordinea celor mai restante mai întâi, ca înainte.

!!! note "Verificarea integrării cu gazda"
    Deschide `/spike` în interfața web după ce containerul pornește. Sondează fiecare montare și CLI (socket Docker, libvirt, restic, qemu-img, rclone) și raportează orice element lipsă, astfel încât să poți confirma că containerul este cablat corect înainte să te bazezi pe el.

## Simplu vs Avansat

Implicit, interfața arată doar elementele esențiale (backup, restaurare, programare). Folosește comutatorul **Simplu / Avansat** din bara laterală pentru a dezvălui controalele pentru experți: retenție, copie off-site, hook-uri pre/post, restaurare la nivel de fișier, notificări, metrici Prometheus și instrumentele de integritate/mentenanță. Este o preferință per browser și oprită implicit, așa că noii veniți primesc o interfață curată, iar utilizatorii avansați primesc totul.

## Pașii următori

- Răsfoiește toate **[Funcționalitățile](features.md)**.
- Adaugă una sau mai multe replici **[Off-site și recuperare](offsite-recovery.md)** (fiecare domeniu poate trimite către mai multe destinații simultan) și salvează-ți kitul de recuperare.
- Clonezi o configurație sau te muți pe o stație nouă? Mută-ți întreaga configurație cu cardul **Export și import setări**. Vezi [Configurare](configuration.md#portable-settings-export-and-import).
- Te-ai blocat? Vezi **[Depanare](troubleshooting.md)**.
