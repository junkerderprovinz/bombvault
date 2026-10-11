# Depanare

Un scurt FAQ. Pentru tabelul complet de depanare pe partea de gazdă pentru VM-prin-SSH (permission-denied, verificarea cheii gazdei, variabile de șablon lipsă și altele), vezi [ghidul de backup VM prin SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md) pe GitHub.

## Ceva nu este cablat corect

Adaugă cardul **Integrare gazdă** în prezentare, la **Personalizează**. Verificarea integrării gazdei sondează fiecare montare și CLI (socket Docker, libvirt, restic, qemu-img, rclone) și raportează orice element lipsă, iar **Verifică acum** o rulează din nou. Începe de aici înainte să presupui o eroare: o montare lipsă sau o gazdă inaccesibilă apare imediat.

## Nu pot ajunge la interfața web

BombVault servește HTTPS din start pe portul `3443` (certificat auto-semnat), deci deschide `https://<your-unraid-ip>:3443`. Acceptă avertismentul de certificat auto-semnat sau pune BombVault în spatele unui reverse proxy cu propriul tău certificat. Dacă rulezi cu `HTTP_ONLY=true`, servește în schimb HTTP simplu pe portul `3000` (destinat utilizării în spatele unui proxy care termină TLS).

## Mi-am pierdut APP_KEY

`APP_KEY` derivă parola depozitului restic. Fără el, fără kitul de recuperare și fără o depunere de cheie în grupul tău, copiile de rezervă criptate nu pot fi recuperate. De aceea prezentarea arată **Salvează-ți kitul de recuperare** până îl descarci. Vezi [Off-site și recuperare](offsite-recovery.md#recovery-kit). Generează o cheie cu `openssl rand -hex 32` și păstreaz-o în afara serverului înainte să te bazezi pe orice copie.

## Am uitat parola de conectare

Alege **Ai uitat parola?** pe pagina de autentificare. BombVault scrie un cod de 12 caractere în jurnalul containerului său (în Unraid: fila Docker, pictograma BombVault, **Logs**). Tastează-l cu o parolă nouă în 10 minute. Resetarea dezactivează și autentificarea în doi pași, elimină toate cheile de acces, revocă toate tokenurile API, cheile MCP și autentificările OAuth, încheie toate celelalte sesiuni și oprește împerecherea telefoanelor. Adaugă din nou cheile de acces la **Setări, Securitate**, pornește tot acolo împerecherea telefoanelor și creează tokenuri și chei noi unde mai ai nevoie de ele. Vezi [Model de securitate](configuration.md#login).

## Copiile s-au oprit și spun că cheia nu mai poate fi deschisă

Acest server a preluat cheia de copii a altui server în Asistentul de urgență, iar `APP_KEY` a fost schimbat după aceea. Cheia preluată este sigilată cu vechiul APP_KEY, așa că BombVault nu o poate deschide și nu face nicio copie, în loc să scrie cu cheia greșită. **Necesită atenție** din prezentare o numește. Pune APP_KEY la loc, cum era, sau încarcă din nou kitul de recuperare pe pagina Restaurare, la **Urgență**.

## Un telefon ajunge pe pagina de autentificare

Un telefon împerecheat primește sesiunea prin grup doar cât timp **Telefoanele se pot împerechea cu cele douăsprezece cuvinte ale grupului** este pornit la **Setări, Securitate**. O resetare a parolei îl oprește. O aplicație mai veche decât serverul nu primește nici ea sesiune până se actualizează. Un server adăugat după adresă cere mereu parola.

## Backupul VM nu se conectează

Backupul VM comunică cu libvirt prin SSH, niciodată o montare.

- Confirmă că SSH este activat pe gazdă și că cheia publică a BombVault este autorizată în `/root/.ssh/authorized_keys` (Setări, Integrări, SSH al gazdei arată cheia și un buton **Testează conexiunea**).
- Pe o rețea `br0.x` personalizată, setează `LIBVIRT_HOST` la IP-ul LAN al Unraid (containerul nu poate ajunge acolo la gazdă prin `host.docker.internal`). Activează **Settings, Docker, Host access to custom networks**.
- Dacă ai schimbat portul SSH al Unraid, setează `LIBVIRT_SSH_PORT` să corespundă.
- Diagnosticul complet pas cu pas (test de accesibilitate, rutare VLAN, `Permission denied (publickey)`, `Host key verification failed`) este în [ghidul de backup VM prin SSH](https://github.com/junkerderprovinz/bombvault/blob/main/docs/vm-backup-ssh-setup.md).

## Un instantaneu VM live nu a rulat

Instantaneele live au nevoie de qemu guest agent instalat în VM și de discul pe `/mnt/cache` (sau `/mnt/diskX`), nu `/mnt/user`. Pe o VM oprită, live revine automat la controlat. Un backup controlat oprește VM-ul, face backup discurilor, apoi îl repornește, așa că este mereu consistent.

## Un backup a eșuat cu "repository is already locked"

De obicei este un lock restic orfan rămas când containerul a fost actualizat sau repornit în mijlocul unei operațiuni. BombVault detectează un lock dovedit orfan, îl curăță forțat și reîncearcă o dată, automat. Dacă persistă, deschide cardul **Verifică copiile de rezervă** de la **Setări, Verificare** și alege **Deblochează** la **Mai mult**, în rândul secțiunii afectate, ca să ștergi manual un lock vechi. O problemă reală tot iese la iveală, nu este ascunsă. După o repornire BombVault așteaptă până când un astfel de lock a stat zece minute fără reîmprospătare. Un restic care încă rulează, de exemplu într-un al doilea BombVault pe același depozit, își reîmprospătează lockul la fiecare cinci minute.

## Copia mea off-site nu s-a întâmplat după un backup

Replicarea off-site este best-effort prin concepție, așa că un sughiț off-site nu face niciodată să eșueze copia locală. Verifică când copiază destinația: cardul ei **Copiază** spune **După fiecare copie de rezervă** sau numește un plan propriu, iar **Setări, Programare, Copii externe pe domenii** poate da unei secțiuni un plan propriu. Folosește **Replică acum** în cardul de copie off-site al secțiunii de la **Setări, Destinații** pentru o rulare la cerere și urmărește antetul de stare din prezentare, care spune **Copierea off-site rulează** cât timp rulează. O copiere eșuată apare la **Necesită atenție**.

## O restaurare s-a oprit înainte de a începe

Înainte ca ceva să fie oprit sau eliminat, restaurarea rulează o verificare de conflicte înainte de zbor: verifică dacă IP-ul static al containerului și porturile publicate ale gazdei sunt libere. Dacă un alt container deține deja unul, se oprește cu un mesaj clar, acționabil, în loc să lase o restaurare pe jumătate terminată. Eliberează portul sau IP-ul în conflict, apoi reîncearcă.

## Un export în clar a eșuat în loc să scrie un fișier

Dacă **Criptează exporturile cu age** este pornit la **Setări, Sistem**, dar nu este setat niciun destinatar valid, un export eșuează cu o eroare clară în loc să scrie text simplu. Adaugă un destinatar valid (o cheie publică age sau o cheie publică SSH) sau oprește criptarea dacă vrei ca exportul să fie text simplu. Vezi [Funcționalități](features.md).

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
- **`ssh-auth`**: serverul a refuzat cheia BombVault. **Conexiune ZFS la acest server**, deschisă cu **Vezi conexiunea** pe pagina Copii de rezervă, arată comanda care o autorizează; rulează-o o dată pe server.

## Un element rămâne la „Învață N/10”

Majoritatea verificărilor de anomalii pornesc după 10 copii reușite ale unui element, iar numărătoarea reîncepe după **Marchează ca așteptată** și după ce selecția elementului s-a schimbat. Un element care nu este programat nu învață, iar un container fără appdata nu are de unde învăța, lucru pe care îl spune insigna lui. **Progresul învățării** de pe pagina Anomalii arată un inel din zece părți pe element.

## Retenția a încetat să șteargă copiile vechi ale unui element

O anomalie critică deschisă le ține: sursa elementului este aproape goală, s-a micșorat brusc sau o copie a stocat din nou cea mai mare parte din datele ei. Deschide anomalia din insigna elementului. Dacă lipsesc date sau au fost criptate, restaurează mai întâi din ultima copie bună la care trimite. Apoi confirmă anomalia sau marcheaz-o ca așteptată dacă schimbarea a fost a ta, iar rularea următoare curăță ca de obicei. Previzualizarea retenției marchează un astfel de element ca păstrat. Pentru un element ZFS doar setul de date numit în anomalie își păstrează copiile vechi, iar celelalte seturi de date din arbore sunt curățate ca de obicei.

## Curățarea manuală spune că unele elemente au fost păstrate

Aceeași cauză: curățarea lasă în pace copiile vechi ale unui element cu o asemenea anomalie și numește elementul în mesajul ei. Tot restul este curățat ca de obicei.

## Importul istoricului spune că un depozit nu a putut fi citit

După actualizare, BombVault citește o dată dimensiunile copiilor anterioare din fiecare depozit. Un depozit pe care nu l-a putut atinge atunci, de exemplu o destinație off-site căzută sau o partajare nemontată, este numărat în cardul **Anomalii** de la **Setări, Verificare** și reîncercat o dată pe zi. Elementele lui învață între timp din copiile noi.

## Avertismentul despre spațiul pe disc nu se potrivește cu panoul Unraid

Pe share-ul de utilizator Unraid (`/mnt/user`) spațiul liber este cel al întregului array, nu al unui singur disc. Depozitele la distanță sunt măsurate doar prin remote-uri rclone care își raportează spațiul liber; depozitele S3, B2, REST și SFTP nu au o valoare și apar ca nemăsurate în cardul **Anomalii**.

## Un asistent IA nu se poate conecta

Pagina [Server MCP](mcp.md#troubleshooting) explică ce înseamnă fiecare cod de stare și fiecare refuz al punctului de conectare MCP și ce poți face.

## Containerul se tot repornește sau pare nesănătos

BombVault raportează sănătos/nesănătos din propriul `/api/health`. Un instrument de auto-vindecare (precum Autoheal) îl poate reporni automat dacă motorul se blochează vreodată. Verifică jurnalul containerului și cardul **Integrare gazdă** pentru cauza de fond.

## Tot blocat?

- Citește paginile complete [Configurare](configuration.md) și [Off-site și recuperare](offsite-recovery.md).
- Întreabă pe [firul de suport Unraid](https://forums.unraid.net/topic/199509-support-junkerderprovinz-bombvault/).
- Deschide o [problemă pe GitHub](https://github.com/junkerderprovinz/bombvault/issues).
