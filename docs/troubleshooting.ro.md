# Depanare

Un scurt FAQ. Pentru tabelul complet de depanare pe partea de gazdă pentru VM-prin-SSH (permission-denied, verificarea cheii gazdei, variabile de șablon lipsă și altele), vezi [ghidul de backup VM prin SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) pe GitHub.

## Ceva nu este cablat corect

Deschide `/spike` în interfața web. Verificarea integrării cu gazda sondează fiecare montare și CLI (socket Docker, libvirt, restic, qemu-img, rclone) și raportează orice element lipsă. Începe aici înainte de a presupune o eroare: o montare lipsă sau o gazdă inaccesibilă apare imediat.

## Nu pot ajunge la interfața web

BombVault servește HTTPS din start pe portul `3443` (certificat auto-semnat), deci deschide `https://<your-unraid-ip>:3443`. Acceptă avertismentul de certificat auto-semnat sau pune BombVault în spatele unui reverse proxy cu propriul tău certificat. Dacă rulezi cu `HTTP_ONLY=true`, servește în schimb HTTP simplu pe portul `3000` (destinat utilizării în spatele unui proxy care termină TLS).

## Mi-am pierdut APP_KEY

`APP_KEY` derivă parola depozitului restic. Fără el (și fără kitul de recuperare a cheii de criptare), backupurile criptate nu pot fi recuperate. De aceea panoul principal insistă să descarci kitul de recuperare. Vezi [Off-site și recuperare](offsite-recovery.md). Generează o cheie cu `openssl rand -hex 32` și stoch-o în afara serverului înainte să te bazezi pe vreun backup.

## Backupul VM nu se conectează

Backupul VM comunică cu libvirt prin SSH, niciodată o montare.

- Confirmă că SSH este activat pe gazdă și că cheia publică a BombVault este autorizată în `/root/.ssh/authorized_keys` (Setări, Sistem, VM Backup over SSH arată cheia și un buton **Test connection**).
- Pe o rețea `br0.x` personalizată, setează `LIBVIRT_HOST` la IP-ul LAN al Unraid (containerul nu poate ajunge acolo la gazdă prin `host.docker.internal`). Activează **Setări, Docker, Host access to custom networks**.
- Dacă ai schimbat portul SSH al Unraid, setează `LIBVIRT_SSH_PORT` să corespundă.
- Diagnosticul complet pas cu pas (test de accesibilitate, rutare VLAN, `Permission denied (publickey)`, `Host key verification failed`) este în [ghidul de backup VM prin SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Un instantaneu VM live nu a rulat

Instantaneele live au nevoie de qemu guest agent instalat în VM și de discul pe `/mnt/cache` (sau `/mnt/diskX`), nu `/mnt/user`. Pe o VM oprită, live revine automat la controlat. Un backup controlat oprește VM-ul, face backup discurilor, apoi îl repornește, așa că este mereu consistent.

## Un backup a eșuat cu "repository is already locked"

Acesta este de obicei un blocaj restic orfan lăsat în urmă când containerul a fost actualizat sau repornit în mijlocul unei operațiuni. BombVault detectează un blocaj dovedibil orfan, îl forțează să se elibereze și reîncearcă o dată, automat. Dacă persistă, folosește **Setări, Integritate și mentenanță, Unlock** pentru domeniul afectat pentru a elibera manual un blocaj rămas. O problemă reală tot iese la suprafață în loc să fie ascunsă.

## Copia mea off-site nu s-a întâmplat după un backup

Replicarea off-site este best-effort prin design, așa că o problemă off-site nu eșuează niciodată backupul local. Verifică programarea off-site pentru acel domeniu (Setări, Programări): o programare goală replică după fiecare backup local, în timp ce o cadență trimite mai rar. Folosește **Replicate now** în fila Off-site pentru o rulare la cerere și urmărește indicatorul de replicare pe panoul principal.

## O restaurare s-a oprit înainte de a începe

Înainte ca ceva să fie oprit sau eliminat, restaurarea rulează o verificare de conflicte înainte de zbor: verifică dacă IP-ul static al containerului și porturile publicate ale gazdei sunt libere. Dacă un alt container deține deja unul, se oprește cu un mesaj clar, acționabil, în loc să lase o restaurare pe jumătate terminată. Eliberează portul sau IP-ul în conflict, apoi reîncearcă.

## Un export în clar a eșuat în loc să scrie un fișier

Dacă criptarea age este activată (Setări) dar niciun destinatar valid nu este setat, un export eșuează cu o eroare clară în loc să scrie text în clar. Adaugă un destinatar valid (o cheie publică age sau o cheie publică SSH), sau dezactivează criptarea dacă intenționezi ca exportul să fie în clar. Vezi [Funcționalități](features.md).

## Un dump de bază de date a eșuat

Un dump eșuat nu duce niciodată la eșec backupul din jurul lui; este consemnat ca o rulare eșuată de sine stătătoare, iar motivul spune ce trebuie reparat.

- **Autentificare refuzată.** Dumpul se autentifică cu variabilele de parolă ale containerului însuși (`POSTGRES_PASSWORD`, `MARIADB_ROOT_PASSWORD`, `MYSQL_ROOT_PASSWORD` sau versiunile lor `_FILE`). Verifică-le pe containerul bazei de date. O variabilă `_FILE` care arată spre un secret pe care utilizatorul containerului nu îl poate citi eșuează la fel.
- **Privilegii lipsă.** Cu o parolă de root aleatoare, dumpul se poate autentifica doar ca utilizator al aplicației, deci conține doar acea bază, iar MySQL 8.4 și mai nou îl poate refuza cu totul. Dă containerului o parolă de root adevărată, sau oprește-i dumpul.
- **Tabelele de sistem au nevoie de o actualizare.** MariaDB refuză dumpul când tabelele sale de sistem vin dintr-o versiune mai veche (eroarea 1558). Adaugă variabila `MARIADB_AUTO_UPGRADE=1` și repornește containerul, sau rulează o dată `mariadb-upgrade` înăuntru.
- **Niciun instrument de dump.** O imagine subțiată sau făcută în casă, fără `pg_dump`, `mysqldump` sau `mariadb-dump`, nu poate fi dumpată. Folosește imaginea oficială, sau oprește dumpul.
- **O limită de timp.** Un dump primește `DB_DUMP_MAX_HOURS` (6 implicit), backupul din jur primește `BACKUP_MAX_HOURS`, iar un dump care nu mai avansează este tăiat după `BACKUP_STALL_HOURS`. În spatele ultimului caz stă de obicei un lacăt ținut de aplicație. Ridică limita care s-a declanșat, sau fă dumpul cât aplicația e liniștită.
- **Containerul este în pauză sau repornește.** Dumpul vorbește cu serverul în funcțiune. Dacă containerul repornește la nesfârșit, jurnalul lui spune de ce.
- **Un dump deteriorat nu a putut fi înlăturat.** Un dump pe care BombVault nu l-a putut termina este șters. Când ștergerea aceea eșuează, dumpul rămâne în listă marcat ca deteriorat și îl poți șterge de acolo.

## Un import a eșuat

Un import oprește containerul, pune deoparte folderul lui de date și lasă imaginea să creeze unul gol în loc. Dacă eșuează un pas de dinaintea importului propriu-zis, folderul vechi este pus la loc de la sine. Dacă eșuează importul, containerul rămâne cu folderul proaspăt, iar cel vechi stă alături ca `<folder de date>.bombvault-before-import-<marcaj de timp>`; mesajul de eroare al rulării numește calea exactă.

Ca să îl pui la loc manual: oprește containerul, redenumește folderul de date curent ca să îl dai la o parte, redenumește folderul păstrat înapoi la numele original și pornește containerul. Pe Unraid, managerul de fișiere din fila Shares face asta.

## Un backup al unui set de date ZFS a eșuat sau a sărit un set {#zfs-datasets}

Fiecare problemă are un cod de motiv între paranteze drepte, iar pagina [Seturi de date ZFS](zfs-datasets.md#reason-codes) le enumeră pe toate cu remedierea. Cele mai frecvente trei:

- **`snapshot-loop`**: instantaneul nu a ajuns la BombVault, pentru că Host Data nu transmite montările noi. Editează containerul, setează Access Mode pentru Host Data la Read/Write - Slave și repornește BombVault.
- **`key-not-loaded`**: un set de date criptat a cărui cheie nu este încărcată este sărit. Încarcă cheia cu `zfs load-key` și montează setul; următorul backup îl include.
- **`ssh-auth`**: serverul a refuzat cheia BombVault. Cardul de conexiune de pe pagina ZFS arată comanda care o autorizează; rulează-o o dată pe server.

## Containerul se tot repornește sau pare nesănătos

BombVault raportează sănătos/nesănătos din propriul `/api/health`. Un instrument de auto-vindecare (precum Autoheal) îl poate reporni automat dacă motorul se blochează vreodată. Verifică jurnalul containerului și raportul `/spike` pentru cauza de bază.

## Tot blocat?

- Citește paginile complete [Configurare](configuration.md) și [Off-site și recuperare](offsite-recovery.md).
- Întreabă pe [firul de suport Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Deschide o [problemă pe GitHub](https://github.com/junkerderprovinz/bombvault/issues).
