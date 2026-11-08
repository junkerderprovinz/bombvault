# Backupuri și elemente

Tot ce salvează BombVault se află într-o singură listă la **Copii de rezervă**: containere, VM-uri, seturi de foldere, unitatea flash, seturi de date ZFS și auto-backupul. Fiecare este un **element** cu pagină proprie. Această pagină parcurge lista, pagina unui element cu cele trei file ale ei și setările pe care le poate avea un element.

![Lista Copii de rezervă cu placa Containere aleasă: fiecare rând își arată ultimele rulări ca o bandă și o singură stare.](assets/screenshots/backups/list-dark.png#only-dark)
![Lista Copii de rezervă cu placa Containere aleasă: fiecare rând își arată ultimele rulări ca o bandă și o singură stare.](assets/screenshots/backups/list-light.png#only-light)

## Ce salvează fiecare secțiune {#what-is-saved}

| Secțiune | Ce se salvează |
|---|---|
| **Containere** | Folderul appdata plus definiția containerului: imagine, variabile, porturi, etichete și volume. Cardul **Foldere de salvat** din fila **Setări** a containerului bifează exact ce foldere acoperă backupul, cu un număr în direct al căilor și o listă cu ce ai lăsat deoparte. Datele pe care un container le ține în afara Host Data, de exemplu într-un ixVolume TrueNAS sau într-un proiect Compose sub `/opt`, nu sunt salvate, iar rularea numește folderele pe care le-a lăsat deoparte. Rularea numește și un folder din Host Data care nu există chiar acum. Căile de sistem precum `/etc/localtime` sau socketul Docker nu contează ca date. |
| **VM-uri** | Imaginile de disc, definiția XML și NVRAM-ul UEFI, cu o oprire controlată sau un instantaneu live, prin SSH. Vezi [Backupuri VM](vm-backup-ssh-setup.md). |
| **Flash** | Doar Unraid. Întregul stick USB flash (`/boot`): sistemul de operare, licența, configurația array-ului, share-urile, rețeaua și configurația pluginurilor. |
| **Auto-backup** | `/config` al BombVault: baza lui de date de setări, autentificările destinațiilor, perechea de chei SSH libvirt, cheia și cheile de gazdă fixate ale serverelor ZFS și certificatele, capturate cu SQLite `VACUUM INTO`, ca baza de date să nu fie niciodată prinsă în mijlocul unei scrieri. |
| **Foldere** | Orice folder de pe server ca set de foldere, de exemplu un share, documentele tale sau o bibliotecă foto, cu modele de excludere opționale. Seturile de foldere au programări, retenție, copii off-site, verificări și teste de restaurare ca orice altceva. |
| **Seturi de date ZFS** | Un set de date și toate seturile de sub el, citite dintr-un singur instantaneu ZFS, ca toate să vină din același moment, și stocate cu restic ca un folder. Un element ZFS poate ține și o replică exactă la nivel de bloc pe un al doilea server ZFS. Vezi [Seturi de date ZFS](zfs-datasets.md). |

Fiecare backup trece prin restic, deci este incremental și deduplicat, iar chiar și discurile mari de VM nu umflă depozitul. BombVault nu își salvează niciodată propriul container și deci nu îl oprește niciodată. O rulare poate fi oprită în siguranță, pentru că restic scrie instantaneul ultimul, așa că o rulare întreruptă lasă date nereferențiate și niciun instantaneu. O blocare rămasă de la o repornire în mijlocul unei rulări este ștearsă și rularea încercată încă o dată, abia după ce blocarea a stat zece minute fără să fie reîmprospătată.

## Lista {#list}

Plăcile de deasupra listei aleg un tip, de exemplu **Containere** sau **ZFS**, iar fiecare placă își numără elementele. O placă selectează, nu ascunde restul paginii.

Fiecare rând arată ultimele paisprezece rulări ale elementului ca pătrățele mici și o singură stare, de exemplu **Protejat**, **Nu este protejat**, **Amânat** sau **Se replică**. Tot restul este în balonul de informații de lângă numele lui, iar cât timp cursorul stă pe rând, acel text apare chiar în rând. Un semn de lângă numele unui container spune că a fost recreat cu altă imagine, alte porturi, variabile sau volume de la ultimul lui backup. Dispare cu următorul backup.

Trei butoane deasupra listei o restrâng:

- **Filtre**: după **Programare** (**Toate**, **Programat**, **Suspendat**), după **Copie de rezervă** (**Cu copie**, **Fără copie**) și, pentru **Containere și VM-uri**, **Instalate** sau **Neinstalat**.
- **Sortare**: după nume, stare, IP, ultimul backup, dimensiune sau programare. Fiecare câmp în afară de IP se sortează în ambele sensuri, de exemplu **Dimensiune, cele mai mari primele** și **Dimensiune, cele mai mici primele**.
- **Caută**: după nume, imagine sau IP.

Elementele care nu mai sunt instalate, dar au încă backupuri, stau deoparte sub **Neinstalat (numai copii de rezervă)**. Restaurează-le sau șterge-le backupurile ca să eliberezi spațiul.

Acțiunea principală a paginii plutește jos în dreapta: **Copiază tot acum** pornește o rulare peste fiecare secțiune pornită. **Mai mult** conține restul:

- **Selectează** începe o selecție, vezi mai jos.
- **Descoperă copii de rezervă** citește ce conțin destinațiile și reconstruiește elemente din asta, de exemplu după o instalare nouă. Un container fără date proprii nu are instantaneu și revine din definiția oglindită lângă depozit. Copiile off-site ale secțiunilor reconstruite așteaptă apoi până preiei backupurile lor ca fiind ale acestui server, vezi [După o reconstruire](offsite-recovery.md#after-a-rebuild).
- **Conexiune ZFS la acest server** deschide testul SSH al secțiunii ZFS, vezi [Seturi de date ZFS](zfs-datasets.md#requirements).
- **Replică tot acum**, cu placa ZFS aleasă, trimite replica fiecărui element ZFS.

![Aceeași listă pe un telefon.](assets/screenshots/backups/list-phone-dark.png#only-dark)
![Aceeași listă pe un telefon.](assets/screenshots/backups/list-phone-light.png#only-light)

## Alegerea mai multor elemente {#select}

Alege rânduri cum alegi fișiere într-un manager de fișiere: Ctrl+clic, Shift+clic, Ctrl+A sau o apăsare lungă pe telefon. Un rând ales arată o bifă în locul pictogramei, iar o bară deasupra listei le numără și oferă **Copiază selectatele**, **Pune programarea pe pauză** și **Reia programarea**, **Selectează tot** și **Golește selecția**. Fiecare rând are și un meniu cu clic dreapta, cu acțiunile lui.

O selecție este salvată pe server, un element după altul, așa că merge mai departe când închizi fila. Ca să restaurezi mai multe elemente împreună, folosește [pagina Restaurare](restore.md#several).

![Două containere alese, cu bara de deasupra listei.](assets/screenshots/backups/bulk-dark.png#only-dark)
![Două containere alese, cu bara de deasupra listei.](assets/screenshots/backups/bulk-light.png#only-light)

## Adăugarea unui folder sau a unui set de date ZFS {#add}

Containerele, VM-urile și unitatea flash apar singure odată ce secțiunea lor este pornită la **Setări, General**. Folderele și seturile de date ZFS le alegi singur, pentru că BombVault nu poate ghici pe care le ai în vedere. **Adaugă** deschide **Ce altceva ar trebui să salveze BombVault?**:

- **Un folder** listează share-urile găsite de BombVault și acceptă orice alt folder de sub Host Data. Dă setului de foldere un nume, de exemplu `Documents`. Numele este fixat după primul backup, pentru că instantaneele sunt stocate sub el.
- **Un set de date ZFS** listează seturile de date ale acestui server. Alege-l pe cel din vârful a ceea ce vrei să salvezi, vezi [Elemente și seturi de date copil](zfs-datasets.md#items-and-children).
- **Configurația sistemului** apare pe o gazdă fără Unraid al cărei Host Data ajunge la `/etc`. Este un set de foldere `/etc` de pornire pe care îl verifici înainte să salvezi.

## Lăsarea unui element deoparte {#dont-back-up}

Un element pe care îl lași deoparte intenționat, de exemplu un container de test, poate fi marcat **Fără copie de rezervă**. Atunci nu mai este socotit neprotejat, iar **Nu mai exclude** îl readuce. **Pune programarea pe pauză** este varianta mai blândă: elementul părăsește programarea și backupul total, poartă o insignă gri **Programare oprită**, iar **Copiază acum** merge în continuare. Setează același comutator ca **În programare** din fila Setări a elementului.

## Pagina unui element {#entry-page}

Un clic pe un rând deschide pagina elementului. Antetul numește elementul și starea lui într-un rând, de exemplu **Ultimul backup acum 3 ore** sau **Amânat, următoarea încercare la 03:15**, și conține acțiunile lui: **Copiază acum**, **Restaurare** și, sub **Mai mult**, cele rare, de exemplu **Exportă cea mai nouă copie**, **Exportă containerul** sau **Exportă VM-ul**, **Leagă de un element mai vechi**, **Șterge toate copiile de rezervă**, **Elimină elementul** și **Șterge copiile de rezervă peste tot**. Un buton înapoi în stânga pictogramei duce la listă. Cât rulează un backup, **Anulează copia** stă lângă progresul lui, iar o rulare anulată este înregistrată ca anulată, nu ca eșuată.

**Șterge copiile de rezervă peste tot** elimină dintr-odată copiile de rezervă ale elementului din fiecare destinație, inclusiv copiile off-site. Fereastra arată fiecare loc și câte copii de rezervă are, iar butonul se deblochează după ce tastezi numele elementului. Un loc setat ca append-only își păstrează copiile și spune asta. Elementul și setările lui rămân, așa că un element pe care nu-l mai salvezi rămâne așa și dispar doar copiile lui de rezervă și istoricul lor.

Pagina are trei file: **Copii de rezervă**, **Setări** și **Verificare**.

### Fila Copii de rezervă {#backups-tab}

Această filă listează instantaneele elementului, cele mai noi primele, cu o bară pe zi. Un backup care se află în mai multe locuri apare o singură dată, marcat cu fiecare destinație care îl conține. Un clic pe un instantaneu îl restaurează, iar un clic pe un loc din interiorul unui instantaneu îl restaurează exact de acolo.

- **Destinații** numește fiecare loc cu backupuri ale acestui element și câte are. Locurile off-site sunt citite doar când apeși **Verificare**, pentru că listarea stocării în cloud durează. Un loc în care elementul nu mai salvează apare ca **Doar copii de rezervă mai vechi** și din el se poate încă restaura.
- Fiecare instantaneu are un meniu: **Adaugă etichetă** sau **Schimbă eticheta**, **Compară cu altă copie**, care listează fișierele apărute, dispărute sau schimbate, **Exportă** ca tar, zip sau un depozit restic propriu, și ștergerea într-un singur loc.
- **Copii SQL ale bazei de date** ale unui container stau într-o listă separată, cu descărcare, salvare într-un folder și import.

Câte instantanee rămân se stabilește la destinație, vezi [Cât timp se păstrează](offsite-recovery.md#keep).

![Fila Copii de rezervă a unui container: o bară pe zi și destinațiile care conțin fiecare instantaneu.](assets/screenshots/backups/entry-backups-dark.png#only-dark)
![Fila Copii de rezervă a unui container: o bară pe zi și destinațiile care conțin fiecare instantaneu.](assets/screenshots/backups/entry-backups-light.png#only-light)

### Fila Setări {#settings-tab}

Fiecare element începe cu **Programare și destinație**:

- **În programare** include elementul în rulările programate sau îl scoate.
- **Programare proprie** dă acestui element un plan propriu, vezi [Un plan propriu pentru un element](schedules.md#own-plan). Fără el, rulează pe planul secțiunii sale sau în backupul total.
- Un rând de butoane, **Local** și câte unul pentru fiecare destinație off-site, spune unde ajung backupurile. **Urmează valoarea implicită** înseamnă că elementul merge unde merge secțiunea lui, iar un clic pe el deschide **Destinații pe secțiuni**. Vezi [Amplasare per element](offsite-recovery.md#placement).
- **Depozit** numește unde sunt scrise backupurile (**Stocat pe**). Acel loc este fixat de la primul backup, copiile se pot schimba oricând.

Cardurile de dedesubt depind de tipul elementului:

| Tip | Carduri |
|---|---|
| Container | **Foldere de salvat** (ce foldere mapate sunt salvate, cu **Omite folderele de cache (CACHEDIR.TAG)**), **Modele de excludere**, **Oprește alte containere**, **Comenzi înainte și după**, **Bază de date**, **Când aplicația e ocupată**, **Actualizează după o copie de rezervă reușită**, **Instantaneu în loc de oprire**, **Nume anterioare** |
| VM | **VM** cu **Metodă** (**Controlat (oprire)** sau **Instantaneu live**) și **Doar blocurile modificate**, vezi [ghidul de backup VM](vm-backup-ssh-setup.md#per-vm-method) |
| Set de foldere | **Folder** și **Modele de excludere** |
| Unitatea flash | **Export ZIP flash**, care scrie fiecare backup flash ca `.zip` simplu într-un folder ales de tine, dintr-un singur pas, ca un instrument de sincronizare să nu vadă niciodată un fișier pe jumătate. Un backup flash care conține mai puțin decât ar trebui păstrează ultimul ZIP pe loc, iar rularea exportului spune asta. |
| Set de date ZFS | seturile de date copil, containerele de oprit și comenzile din jurul instantaneului, **Replică** și **Testează accesul la instantanee**, vezi [Seturi de date ZFS](zfs-datasets.md) |

Setările se salvează singure pe măsură ce le schimbi.

![Fila Setări a unui container, cu Programare proprie în primul card.](assets/screenshots/backups/entry-settings-dark.png#only-dark)
![Fila Setări a unui container, cu Programare proprie în primul card.](assets/screenshots/backups/entry-settings-light.png#only-light)

### Fila Verificare {#verify-tab}

- **Verificări** arată verificarea de restaurare care urmează după primul backup. **Verifică restaurarea** o rulează din nou pe cel mai nou backup: BombVault restaurează un eșantion de până la 100 de fișiere și 256 MiB într-un folder temporar și pune restic să citească fiecare fișier înapoi față de hash-urile lui.
- **Test de pornire**, la un container, restaurează cel mai nou backup într-o copie izolată și o pornește: un nume care începe cu `bombvault-test-`, o rețea Docker internă proprie, fără porturi publicate și fără drum spre LAN, 1 CPU și 2 GiB de memorie. Trece dacă copia raportează că e sănătoasă, răspunde pe primul port expus sau, dacă nu se întâmplă niciuna, rămâne în execuție. Originalul nu este atins niciodată, iar copia, rețeaua ei și datele ei sunt șterse după aceea, și după o repornire în mijlocul unui test. Containerele pe rețeaua gazdei, cele privilegiate, cele cu dispozitive și cele care au nevoie de alt container sunt arătate ca netestabile. **Test de pornire** de la **Setări, Verificare, Verificări automate** testează câte un container pe rulare, pe cel testat cel mai demult primul.
- **Anomalii** stabilește sensibilitatea proprie a acestui element și nivelul lui de notificare și arată constatările lui deschise.
- **Dimensiune pe dosar** arată ce ocupă spațiul în cel mai nou backup și cât a adus ultimul backup, un nivel pe rând.
- **Rulări recente** listează ultimele rulări cu începutul, sfârșitul și durata. Un clic arată detaliile.

![Fila Verificare a unui container cu Dimensiune pe dosar deschis.](assets/screenshots/backups/entry-verify-dark.png#only-dark)
![Fila Verificare a unui container cu Dimensiune pe dosar deschis.](assets/screenshots/backups/entry-verify-light.png#only-light)

## Modele de excludere {#excludes}

Cardul **Modele de excludere** primește un model pe rând. Scrie căile așa cum le vezi în interiorul containerului, de exemplu `/config/Library/Application Support/Plex Media Server/Cache`, sau un nume simplu ca `.git`, care se potrivește la orice adâncime. La un set de foldere, scrie calea așa cum o cunoaște gazda, de exemplu `/mnt/user/data/cache`. O previzualizare sub câmp arată în ce se rezolvă fiecare rând și avertizează când un rând nu se potrivește cu nimic sau ar exclude totul. Un rând cu `/` sau `*`, sau unul care numește un întreg folder salvat, este ignorat, așa că o excludere nu poate goli niciodată un backup. Ca să lași deoparte un folder întreg, debifează-l în schimb la **Foldere de salvat**.

Pentru imaginile cunoscute, cum sunt Plex, Jellyfin, Sonarr, Radarr, Immich și Nextcloud, cardul oferă folderele pe care aplicația le umple din nou singură, de exemplu cache-uri, jurnale și miniaturi. Nimic nu este exclus până nu apeși **Exclude selecția**.

Același card poartă avertismente pe care o scanare de foldere nu le poate vedea. Immich, de exemplu, ține albumele, fețele și datele într-o bază de date PostgreSQL dintr-un container separat, așa că un backup de fișiere doar al containerului Immich restaurează pozele fără nimic din toate acestea, iar restaurarea pare că a mers. Salvează și containerul bazei de date.

## Dumpuri de baze de date {#database-dumps}

Containerele PostgreSQL, MySQL și MariaDB pe care BombVault le recunoaște sunt dumpate înainte de fiecare backup, de pe serverul care rulează, într-un punct de restaurare propriu lângă fișiere. Asta acoperă imaginile oficiale, PostGIS, TimescaleDB, pgvector, pgautoupgrade, imaginile de bază de date ale Immich, imaginile MariaDB de la linuxserver, yobasystems și jc21 și mysql-server de la Oracle. Un container care doar seamănă cu o bază de date primește aceeași opțiune, oprită până o alegi.

Dumpul curge direct în depozit și nu este scris niciodată pe disc. Credențialele vin din variabilele proprii ale containerului, inclusiv secretele `*_FILE`, și nu îl părăsesc niciodată. Cardul **Bază de date** din fila Setări a containerului spune dacă folderul de date este salvat oprit, copiat în execuție sau nesalvat, și pornește sau oprește dumpul pentru acest container. Eticheta `bombvault.dbdump=false` face același lucru și are prioritate față de card. **Copii SQL automate ale bazelor de date** de la **Setări, General** îl comută pentru toate.

Un dump eșuat nu face să eșueze backupul. Apare ca o rulare eșuată separată, cu motivul, și trimite o notificare. Dumpurile nu sunt niciodată reîncărcate singure: descarcă unul, simplu sau comprimat, salvează-l într-un folder sau importă-l într-o bază de date tocmai pornită cu un clic în fila **Copii de rezervă**. Ca să recuperezi un dump fără BombVault, vezi [Recuperarea unui dump de bază de date](offsite-recovery.md#database-dumps). Când unul eșuează, vezi [Un dump de bază de date a eșuat](troubleshooting.md#a-database-dump-failed).

## Comenzi și alte containere {#commands}

- **Comenzi înainte și după** rulează în interiorul containerului cu `sh -c`, de exemplu ca să scrie un cache pe disc. O comandă care eșuează înainte de backup îl oprește. O comandă de dinaintea backupului care iese cu codul 75 amână în schimb un backup programat, vezi mai jos.
- **Oprește alte containere** numește containere de oprit cât acesta este salvat, de exemplu baza lui de date. După aceea pornesc din nou în ordinea `depends_on` din Compose, iar implicit fiecare trebuie să raporteze că e sănătos înainte ca următorul să pornească. Așteptarea și limita ei de timp se află la **Repornire după backup** în cardul **Ordine** de la **Setări, Programare**.
- Un container care împarte spațiul de nume de rețea, PID sau IPC cu unul care este oprit, prin `--net=container:` sau prin `network_mode: service:` din Compose, se oprește și pornește odată cu el chiar dacă nu este pe listă, și pornește după containerul la care se alătură. Un container repornit primește o rețea nouă, iar unul lăsat să ruleze pe rețeaua veche ar avea doar loopbackul lui. Lista însăși rămâne cum ai setat-o. Dacă un astfel de container nu a putut fi oprit, rularea spune că nu are rețea până nu este repornit. La fel e și pentru containerul BombVault, care nu se oprește niciodată singur și nu poate fi pus pe listă.
- Un container care nu mai rulează după aceea este numit în rulare și într-o notificare, și atunci când backupul însuși a eșuat.

## Când aplicația e ocupată {#app-busy}

Un backup oprește containerul, așa că unul programat poate aștepta cât timp cineva folosește aplicația și poate încerca din nou singur. Cardul **Când aplicația e ocupată** din fila Setări a unui container îl configurează:

- **Verificare a activității** întreabă Plex, Jellyfin sau Emby, înainte de un backup programat, dacă se uită cineva. Plex este întrebat cu tokenul din propriul `Preferences.xml`, așa că nu ai nimic de introdus. Jellyfin și Emby primesc o **Cheie API**, creată în panoul serverului la API keys.
- **Ignoră sesiunile în pauză după (min)** lasă un film oprit pentru noapte să nu mai conteze.
- **Fă copia când serverul nu răspunde** hotărăște ce se întâmplă când serverul nu poate fi întrebat: pornit, backupul rulează și jurnalul spune asta. Oprit, așteaptă ca și cum s-ar uita cineva.
- **Așteaptă cât timp containerul trimite date** este pentru aplicații fără o astfel de verificare, de exemplu un server de jocuri sau un downloader. Backupul așteaptă cât timp containerul a trimis mai mult de **Ocupat de la (KiB/s)** la orice citire din ultimele **Liniștit timp de (min)** minute.
- **Încearcă din nou la fiecare (min)**, **Încearcă doar într-un interval orar** cu **Încearcă între** și **Așteaptă cel mult (ore)** stabilesc cum merg încercările.
- **La sfârșitul așteptării** fie renunță, ceea ce arată backupul sub **Necesită atenție** cu **Încearcă din nou**, fie **Fă copia oricum**.

Un backup care așteaptă este înregistrat ca **Amânat**, în chihlimbar, iar restul rulării merge mai departe. Prezentarea, lista și elementul îl arată cu următoarea încercare, iar alerta de întârziere și starea protecției îl așteaptă până la termenul lui. Primești o notificare când este amânat și una când se termină, nu una la fiecare încercare. O încercare așteaptă cât rulează alt backup, iar o așteptare continuă după o repornire. Un backup pornit de tine rulează mereu pe loc, fie cu **Copiază acum**, **Încearcă din nou**, o selecție, API sau MCP, iar unul care trece încheie o așteptare aflată în curs.

Fila Setări a containerului are și o variantă mai veche, mai simplă: **Așteaptă până când aplicația e inactivă** cu **Așteaptă cel mult (ore)**. Backupul pornește de îndată ce aplicația este inactivă, lucru definit de cardul **Inactivitate înainte de copie** de la **Setări, Programare**. Membrii unui stack Compose scadenți în aceeași rulare așteaptă împreună, iar backupul care așteaptă nu ține nicio blocare, așa că celelalte containere merg înainte.

![Cardul Când aplicația e ocupată al unui container Plex, cu verificarea activității configurată.](assets/screenshots/backups/app-busy-dark.png#only-dark)
![Cardul Când aplicația e ocupată al unui container Plex, cu verificarea activității configurată.](assets/screenshots/backups/app-busy-light.png#only-light)

### Exemplu: Plex așteaptă cât se uită cineva {#example-plex}

Backupul total rulează în fiecare noapte la 03:00. Nimeni nu ar trebui să vadă un film oprindu-se pentru că Plex a căzut pentru backup, dar backupul tot trebuie să se întâmple până dimineață.

1. Deschide **Copii de rezervă**, apoi elementul `plex`, apoi fila lui **Setări**.
2. La **Când aplicația e ocupată**, setează **Verificare a activității** pe Plex. Nu ai nimic de scris.
3. Setează **Ignoră sesiunile în pauză după (min)** pe `30`, ca un film pus pe pauză la miezul nopții să nu mai conteze la 00:30.
4. Lasă **Fă copia când serverul nu răspunde** pornit.
5. Setează **Încearcă din nou la fiecare (min)** pe `15`.
6. Pornește **Încearcă doar într-un interval orar** și setează **Încearcă între** pe `22:00` și `06:00`.
7. Setează **Așteaptă cel mult (ore)** pe `3` și **La sfârșitul așteptării** pe **Fă copia oricum**.

Ce se întâmplă într-o noapte în care cineva se uită încă la 03:00: BombVault întreabă Plex, găsește o sesiune care redă și înregistrează backupul ca **Amânat**, cu următoarea încercare la 03:15. Celelalte containere ale rulării sunt salvate ca de obicei. Când filmul se termină la 04:10, încercarea de la 04:15 găsește Plex inactiv și îl salvează, iar așteptarea se încheie cu o notificare. Dacă filmul ar rula încă la 06:00, așteptarea ar ajunge la capăt și Plex ar fi salvat oricum, așa că backupurile lui nu rămân niciodată mai mult de trei ore în urmă.

Pentru propria verificare, fă **Comandă pre-backup** să iasă cu codul 75 cât aplicația este ocupată, de exemplu `[ "$(cat /tmp/players)" = 0 ] || exit 75`. Mai multe simptome și remedieri se află în [Depanare](troubleshooting.md#deferred-backups).

## Exporturi în clar {#exports}

restic rămâne motorul, dar un export îți dă o copie care nu are nevoie nici de BombVault, nici de restic.

- **Exportă containerul** și **Exportă VM-ul** sub **Mai mult** pe pagina unui element scriu o copie așa cum este chiar acum, lângă depozit: folderele de backup ale containerului plus șablonul lui Unraid, sau imaginile de disc ale VM-ului plus definiția lui, restaurabile cu `virsh define`.
- **Exportă** pe un instantaneu scrie un backup stocat în folderul de export de pe server, ca tar, zip sau un depozit restic propriu, și poate lua un instantaneu sau pe toate. Un backup flash și un auto-backup conțin chei, așa că exportul unuia cere o parolă de conectare și o autentificare chiar pe instanță.

Exporturile sunt implicit fișiere simple. **Criptează exporturile cu age** de la **Setări, Sistem** le sigilează, inclusiv ZIP-ul flash, pentru destinatarii pe care îi listezi, fiecare o cheie publică age sau o cheie publică SSH. Le decriptezi mai târziu în afara serverului, cu cheia privată corespunzătoare. Cu criptarea pornită și fără niciun destinatar valid, un export eșuează cu o eroare clară, în loc să scrie vreodată fișiere simple.

## Ce îți spune o rulare {#run-notes}

- **Durata.** Fiecare rulare arată începutul, sfârșitul și durata, iar fiecare element are propriile **Rulări recente**.
- **De ce a fost lent un backup.** Cât rulează un backup, BombVault urmărește cât de ocupate sunt CPU, discurile și rețeaua. Când un backup durează mult mai mult decât de obicei și un singur lucru a fost clar la limită, rularea spune asta, de exemplu „Discul țintă disk1 a fost ocupat 98%”. Altfel nu spune nimic.
- **Progresul verificărilor și al curățărilor.** Jurnalul de activitate și cardul **Verifică copiile de rezervă** arată cât a numărat restic, de exemplu *12 din 47 pachete*, cu timpul rămas în acel pas de îndată ce există destule date ca să fie estimat.
- **Fișiere pe care nu le-a putut citi.** Un backup care a trecut, dar a lăsat ceva pe dinafară, își face rândul portocaliu și numește ce a fost lăsat.
