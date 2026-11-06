# Funcționalități

Un scurt catalog cu ce face BombVault. Fiecare rând trimite la pagina care îl explică.

## Sfera backupului {#backup-scope}

- **Containere**: folderul appdata și definiția containerului, cu o alegere a folderelor pentru fiecare container. [Backupuri și elemente](backups.md#what-is-saved)
- **VM-uri**: imagini de disc, XML și NVRAM UEFI prin SSH, cu o oprire controlată, un instantaneu live sau doar blocurile modificate. VM-urile cu instantanee Unraid și discuri pe zvol-uri sunt acoperite complet. [Backupuri VM](vm-backup-ssh-setup.md)
- **Flash**: întregul stick USB flash Unraid, cu un export ZIP programat. [Backupuri și elemente](backups.md#settings-tab)
- **Auto-backup**: setările și autentificările proprii ale BombVault. [Asistentul de urgență](offsite-recovery.md#emergency)
- **Foldere**: orice folder de pe server ca set de foldere. [Adăugarea unui folder](backups.md#add)
- **Seturi de date ZFS**: un arbore de seturi de date citit dintr-un singur instantaneu, cu fișiere individuale restaurabile și o replică opțională pe un al doilea server ZFS. [Seturi de date ZFS](zfs-datasets.md)
- **Dumpuri de baze de date**: PostgreSQL, MySQL și MariaDB, dumpate de pe serverul care rulează într-un punct de restaurare propriu. [Dumpuri de baze de date](backups.md#database-dumps)

## Backupuri și paginile elementelor {#backups}

- **O singură listă** cu plăci, **Filtre**, **Sortare** și **Caută**, și ultimele paisprezece rulări ale fiecărui rând ca o bandă. [Lista](backups.md#list)
- **Alegi ca într-un manager de fișiere** și salvezi sau pui pe pauză o selecție pe server. [Alegerea mai multor elemente](backups.md#select)
- **O pagină pe element** cu **Copii de rezervă**, **Setări** și **Verificare**. [Pagina unui element](backups.md#entry-page)
- **Etichete și comparare** pentru instantanee și **Exportă** ca tar, zip sau un depozit restic. [Fila Copii de rezervă](backups.md#backups-tab)
- **Modele de excludere** cu previzualizare în direct și excluderi recomandate pentru aplicațiile cunoscute. [Modele de excludere](backups.md#excludes)
- **Comenzi înainte și după** și **Oprește alte containere** cu o repornire condiționată de sănătate, în ordinea dependențelor. [Comenzi și alte containere](backups.md#commands)
- **Când aplicația e ocupată**: un backup programat așteaptă Plex, Jellyfin, Emby, propria ta verificare sau trafic liniștit și încearcă din nou singur. [Când aplicația e ocupată](backups.md#app-busy)
- **Instantaneu în loc de oprire**: un container pe ZFS este oprit doar câteva secunde. [Programări](schedules.md#snapshot-instead-of-stop)
- **Exporturi în clar**, opțional sigilate cu age. [Exporturi în clar](backups.md#exports)

## Restaurare {#restore}

- **O pagină și o fereastră** pentru fiecare tip, cu backupul recomandat preselectat. [Restaurare](restore.md)
- **Lângă original** implicit, iar **Înlocuiește originalul** salvează mai întâi starea actuală. [Fereastra de restaurare](restore.md#window)
- **Fișiere individuale** într-un folder sau înapoi la locul lor. [Readu un singur fișier](restore.md#example-single-file)
- **Mai multe elemente deodată**, fiecare din backupul ei recomandat sau dintr-un singur moment. [Restaurarea mai multor elemente](restore.md#several)
- **Stack-uri Compose întregi**, pornite în ordinea dependențelor. [Restaurarea unui stack](restore.md#stacks)
- **Containerele și VM-urile sunt recreate** pe imaginea pe care rulau, cu GPU, limite și legături. [Ce revine](restore.md#kinds)
- **Verificări înainte de restaurare** și o listă cu ce schimbă ea. [Fereastra de restaurare](restore.md#window)
- **Flash ca ZIP** și câte un plugin înapoi în unitatea flash aflată în uz. [Ce revine](restore.md#kinds)
- **ZFS** în setul de date cu un instantaneu de siguranță, într-un folder sau într-un set de date nou cu proprietățile lui. [Seturi de date ZFS](zfs-datasets.md#restore)
- **Asistent de urgență** pentru un server pierdut, cu kitul de recuperare, prin grup sau cu cheia scrisă de mână. [Asistentul de urgență](offsite-recovery.md#emergency)
- **Dintr-un folder vechi de copii** și **Import din pluginul Appdata.Backup**. [Alte surse](restore.md#other-sources)

## Stocare și programare {#storage-scheduling}

- **Destinații** de orice fel: foldere locale, SMB, WebDAV, SFTP, rest-server, S3, drive-uri cloud și orice remote rclone. [Destinații](offsite-recovery.md#destinations)
- **Autentificări**: o singură listă cu o valoare implicită. [Autentificări](offsite-recovery.md#sign-ins)
- **Foldere proprii** pe acest server sau într-o destinație. [Folderul propriu al unei secțiuni](offsite-recovery.md#own-folder)
- **3-2-1 ca flux** în care fiecare rând este un comutator, și **amplasare per element**. [Amplasare per element](offsite-recovery.md#placement)
- **Copii off-site** în mai multe destinații pe secțiune, cu limite de bandă, comprimare, clase de stocare la rece și o încetinire cât Plex face streaming. [Copii off-site](offsite-recovery.md#off-site-copies)
- **Depozite directe**, listate sub **Se salvează direct aici**. [Depozite directe](offsite-recovery.md#direct-repositories)
- **Retenție ca linie a timpului**: **Scurt**, **Echilibrat**, **Lung** sau **Valori proprii** pe destinație, cu reguli pe secțiune. [Cât timp se păstrează](offsite-recovery.md#keep)
- **Mută istoricul unei secțiuni** și **Elimină destinația** cu sau fără backupurile ei. [Mutare și eliminare](offsite-recovery.md#move-remove)
- **Programări care se citesc ca propoziții**, banda **Săptămâna**, un backup total, planuri pe secțiuni și un plan propriu pe element. [Programări](schedules.md)
- **Recuperarea backupurilor ratate**, o ordine manuală și actualizări după un backup reușit. [Programări](schedules.md)

## Perspectivă, verificare și monitorizare {#insight-verification-monitoring}

- **Prezentarea**: antet de stare, **Necesită atenție**, **Starea protecției**, prognoza stocării, istoric și carduri opționale. [Prezentare](overview.md)
- **Detectarea anomaliilor** pe element, care păstrează și backupurile vechi când o sursă se prăbușește. [Anomalii](anomalies.md)
- **Verificare de restaurare** după primul backup și **Test de pornire** într-o copie izolată. [Fila Verificare](backups.md#verify-tab)
- **Verificări programate și restaurări de test** din copia off-site. [Teste DR](offsite-recovery.md#dr-drills)
- **Dimensiune pe dosar** și de ce a fost lent un backup. [Ce îți spune o rulare](backups.md#run-notes)
- **Notificări**: webhook, Gotify, Matrix, e-mail, Apprise, Unraid și Healthchecks, cu un rezumat săptămânal și un raport de întârziere. [Notificări](notifications.md)
- **Prometheus**, un dispozitiv Home Assistant, un API HTTP și mDNS. [API și integrări](api.md)
- **Raport de eroare**: un ZIP cu date șterse pentru un issue GitHub. [Depanare](troubleshooting.md#still-stuck)

## Protecție împotriva ransomware {#ransomware-protection}

- **Protecție împotriva ștergerii (append-only)**, impusă de partea îndepărtată și dovedită printr-un test de manipulare. [Protecție împotriva ștergerii](offsite-recovery.md#delete-protection-append-only)
- **Acceptă fără protecție împotriva ștergerii** pentru un loc care nu poate refuza o ștergere. [Protecție împotriva ștergerii](offsite-recovery.md#delete-protection-append-only)
- **Ștergerea definitivă a backupurilor cere propria ta conectare** când este setată o parolă de conectare, iar un telefon asociat nu o poate face. [Model de securitate](configuration.md#security-model)
- **Kit de recuperare**, opțional sigilat cu age, și un **păstrător de cheie** în grupul tău. [Kit de recuperare](offsite-recovery.md#recovery-kit), [Păstrător de cheie](offsite-recovery.md#key-keeper)

## Mai multe servere {#several-servers}

- **Împerechere prin douăsprezece cuvinte**, criptată cap la cap, direct sau printr-un relay. [Împerecherea instanțelor](offsite-recovery.md#pairing)
- **Pagina Instanțe** cu grupul ca rețea și rolurile **Receptor**, **Preluator**, **Server ZFS** și **Păstrător de cheie**. [Instanțe și roluri](offsite-recovery.md#roles)
- **Un server receptor** configurat pentru tine, în modul append-only. [Server receptor](offsite-recovery.md#receiving-server)

## Asistenți IA (MCP) {#mcp}

Un server MCP încorporat lasă un asistent să citească starea backupurilor, acoperirea, istoricul rulărilor și punctele de restaurare și, cu o cheie care permite asta, să pornească un backup. Restaurările, ștergerile, curățarea (prune) și setările rămân în interfața web. [Server MCP](mcp.md)

## Aplicații și însoțitori {#apps}

- **Aplicația Android**: fiecare server al grupului tău pe telefon. [Aplicația Android](android.md)
- **BombVault Widget**: o placă pe Dashboardul Unraid. [Aplicații](settings.md#apps)
- **Jurnal de activitate încorporabil** pentru dashboarduri precum Homepage sau Heimdall. [Widget pentru dashboard](api.md#dashboard-widget)

## Altele {#other}

- **42 de limbi**, trei teme și etichete, culori, colțuri și animație la alegerea ta. [Aspect](settings.md#look)
- **Setări care se salvează singure**, cu o căutare în toate cele paisprezece plăci. [Setări](settings.md)
- **Noutăți** după o actualizare, cu puncte verzi pe ce s-a schimbat. [Info](settings.md#info)
- **HTTPS din start** și un healthcheck pentru instrumentele de auto-reparare. [Configurare](configuration.md#network)
