# Restaurare

Fiecare restaurare începe pe pagina **Restaurare**: alege o secțiune, caută ce vrei să readuci și alege un backup. O singură fereastră restaurează orice fel de element și întreabă pas cu pas. Implicit nimic nu se schimbă la original, pentru că backupul revine lângă el.

![Pagina Restaurare cu secțiunile ca plăci, căutarea și cardul Stack-uri.](assets/screenshots/restore/page-dark.png#only-dark)
![Pagina Restaurare cu secțiunile ca plăci, căutarea și cardul Stack-uri.](assets/screenshots/restore/page-light.png#only-light)

## Pagina Restaurare {#page}

- **Secțiuni** listează secțiunile ca plăci, fiecare cu numărul de elemente care au instantanee.
- **Ce vrei să readuci?** caută în elementele secțiunii alese. Fiecare rând arată cel mai nou instantaneu al lui, iar **Deschide elementul** duce la pagina elementului.
- **Găsește un fișier** caută un fișier sau un folder după nume în fiecare backup, vezi [Găsirea unui fișier](#find-a-file).
- **Stack-uri** listează proiectele Compose cu cel puțin două containere salvate, de exemplu `immich` cu baza de date și Redis. Căutarea găsește un stack după nume sau după oricare dintre containerele lui.
- **În desfășurare** arată o restaurare cât rulează, și după ce ai închis fereastra ei.
- **Urgență** pornește asistentul de urgență pentru un server care a dispărut sau a fost configurat din nou, vezi [Asistentul de urgență](offsite-recovery.md#emergency).
- **Alte surse** conține **Dintr-un folder vechi de copii**, pentru backupurile unui alt BombVault, **Import din pluginul Appdata.Backup** și **Sarcini din Duplicati sau Duplicacy**.
- **Alte servere** listează celelalte servere ale unui depozit comun din care face parte acest server, vezi [Alte servere](#other-servers).

![Pagina Restaurare pe un telefon.](assets/screenshots/restore/page-phone-dark.png#only-dark)
![Pagina Restaurare pe un telefon.](assets/screenshots/restore/page-phone-light.png#only-light)

## Fereastra de restaurare {#window}

Un clic pe un element deschide **Restaurare** cu numele lui. Întreabă pas cu pas, iar fiecare pas poate fi schimbat înainte să pornești:

1. **Copie de rezervă**: instantaneul recomandat este preselectat. Este cel mai nou sau, după o anomalie deschisă, ultimul dinaintea ei. Backupul făcut înaintea unei restaurări anterioare păstrează starea pe care acea restaurare a înlocuit-o, așa că este sărit, dacă nu există altul. Cele mai vechi sunt în listă, iar **Compară** arată ce diferă între două dintre ele.
2. **Sursă**: de unde vin fișierele, când un instantaneu se află în mai multe locuri. **Local** este cel mai rapid.
3. **Ce**: **Toate** sau **Fișiere individuale**, unde bifezi fișierele și folderele dorite.
4. **Unde**: ținta. Alegerile depind de tipul elementului:

    | Țintă | Ce face |
    |---|---|
    | **Lângă original** | Implicit. Un container revine ca un al doilea container numit `<name>-restored`, cu porturi proprii, discurile și NVRAM-ul unei VM ajung într-un folder numit `<folder>-restored`, iar un set de foldere revine ca `<path>-restored`. Nimic nu se schimbă la original. |
    | **Înlocuiește originalul** | Suprascrie ce există acum. Cu **Fă mai întâi o copie a stării actuale**, pornit implicit, BombVault salvează mai întâi starea de azi, ca să te poți întoarce. Dacă acel backup eșuează, restaurarea nu pornește. Asta merge și pentru un container sau un VM care nu mai este pe server: folderele peste care scrie restaurarea primesc un backup separat. |
    | **Într-un folder** | Doar fișierele, pentru răsfoit. |
    | **Descarcă ca ZIP** | Unitatea flash, pentru un stick nou. BombVault nu scrie niciodată în unitatea flash aflată în uz. |
    | **Într-un set de date nou** și **Restaurează în setul de date** | Seturi de date ZFS, vezi [Seturi de date ZFS](zfs-datasets.md#restore). |

5. **Ce se întâmplă** spune în cuvinte ce va face restaurarea, iar o scenă arată ținta aleasă.

Înainte ca **Restaurare** să se deblocheze, fereastra verifică dacă depozitul răspunde, dacă cheia îl deschide, dacă instantaneul există și dacă ținta are loc. Cât timp o verificare eșuează, butonul rămâne blocat, iar (i) din el spune care.

Fereastra arată și ce schimbă restaurarea față de ce există acum: fișiere noi, înlocuite și neschimbate, cu lista la cerere, plus fișierele de la țintă care nu sunt în backup și rămân unde sunt. restic calculează asta ca simulare, din dimensiune și data modificării, fără să citească fișierele, iar un arbore foarte mare se oprește după 30 de secunde și spune asta. Pentru containere și VM-uri compară setările pe care restaurarea le recreează cu cele care rulează: imagine și etichetă, porturi, nume de variabile și volume, sau memorie, vCPU, discuri și rețea.

Unele lucruri pot opri o restaurare înainte să schimbe ceva. IP-ul static al unui container și porturile publicate trebuie să fie libere, iar rețeaua lui trebuie să existe. Pe Unraid, orice restaurare pe loc refuză un folder care nu este pe un pool sau share montat, pentru că fișierele ar ajunge în memorie și ar dispărea după o repornire. O restaurare pe loc numește fiecare alt container al cărui bind mount ajunge într-un folder în care scrie, de exemplu „această cale este folosită și de nextcloud-db”, dar doar avertizează.

Câteva opțiuni apar unde se aplică: **Lasă oprit după aceea**, ca să te uiți la un container sau la o VM înainte să ruleze din nou, și **Restaurează fără GPU și runtime** pe o gazdă care nu are GPU-ul sau runtime-ul cu care a fost făcut backupul.

Restaurarea rulează în fundal. Poți închide fereastra, iar progresul apare și la **În desfășurare**. O restaurare lungă arată un procent și poate fi anulată, iar una anulată este înregistrată ca anulată, nu ca eșuată. O restaurare anulată cât containerul se oprește, sau una care nu poate elimina containerul, încă nu a restaurat nimic, așa că containerul pornește din nou. Un container care își ia volumele de la altul cu `--volumes-from` le păstrează, iar restaurarea refuză să înceapă cât acel alt container nu există.

![Fereastra de restaurare la pasul Ce, cu fișiere individuale bifate.](assets/screenshots/restore/window-what-dark.png#only-dark)
![Fereastra de restaurare la pasul Ce, cu fișiere individuale bifate.](assets/screenshots/restore/window-what-light.png#only-light)

### Unde ajunge o restaurare într-un folder {#folders}

**Foldere de restaurare** de la **Setări, Restaurare** stabilește **Folder de restaurare implicit**, cu un **Subfolder pentru fiecare secțiune** dacă vrei, și o **Excepție** pentru o secțiune. Fișierele alese ajung direct în folderul țintă: un fișier revine ca acel fișier, un folder ca folderul cu conținutul lui, iar alegerile din foldere diferite păstrează doar calea de sub folderul pe care îl au în comun. BombVault refuză o țintă care nu este pe stocare montată, pentru că pe Unraid un astfel de folder trăiește în memorie și dispare după o repornire, și nu restaurează niciodată în propriul folder de date.

## Exemplu: readu un singur fișier {#example-single-file}

Cineva a editat aseară `config.php` din Nextcloud și Nextcloud nu mai pornește. Backupul de noapte de la 03:00 are încă fișierul bun. Vrei doar acel fișier, nu tot containerul.

1. Deschide **Restaurare**, alege placa **Containere** și scrie `nextcloud` în **Ce vrei să readuci?**.
2. Fă clic pe `nextcloud`. Fereastra se deschide cu backupul de la 03:00 preselectat ca recomandat.
3. Lasă **Sursă** pe **Local**.
4. La **Ce**, alege **Fișiere individuale**. Deschide folderele până la `config` și bifează `config.php`.
5. La **Unde**, alege **Într-un folder** și alege un folder, de exemplu `user/restore`, care este `/mnt/user/restore` pe gazdă.
6. Citește **Ce se întâmplă**: fișierul ajunge în folderul țintă, iar containerul nu se schimbă. Apasă **Restaurare**.

Fereastra se încheie cu **Gata: fișierele sunt în /mnt/user/restore**. Găsești fișierul ca `/mnt/user/restore/config.php`, fără folderele de deasupra. Compară-l cu cel stricat, copiază-l înapoi în folderul appdata și repornește Nextcloud.

Ca să pui fișierul direct la loc, alege la pasul 5 **Înlocuiește originalul**. Ține **Fă mai întâi o copie a stării actuale** pornit, ca BombVault să salveze starea stricată înainte să scrie, scrie `nextcloud` pentru confirmare și apasă **Restaurare**. Nextcloud este oprit cât fișierul ales este scris înapoi la locul lui și pornit din nou după aceea, și se scriu doar fișierele bifate de tine. O restaurare care eșuează la jumătate lasă Nextcloud oprit, pentru că fișierul pe care îl citește poate fi scris doar pe jumătate, iar rularea spune asta.

![Fereastra de restaurare la Ce se întâmplă, cu Înlocuiește originalul ales.](assets/screenshots/restore/window-replace-dark.png#only-dark)
![Fereastra de restaurare la Ce se întâmplă, cu Înlocuiește originalul ales.](assets/screenshots/restore/window-replace-light.png#only-light)

## Găsirea unui fișier {#find-a-file}

Când știi numele unui fișier, dar nu și ce element sau ce backup îl mai are, scrie numele în **Găsește un fișier** și apasă **Caută**. Textul simplu se potrivește oriunde în nume și nu ține cont de majuscule, așa că `invoice` găsește `Invoice-2024.pdf`. Cu `*` sau `?`, modelul trebuie să se potrivească cu tot numele, așa că `*.pdf` găsește fiecare PDF. Scrie doar numele, fără folder.

Căutarea citește fiecare backup al containerelor, al seturilor de foldere, al seturilor de date ZFS și al unității flash. Fiecare rezultat arată elementul, calea și fiecare backup care conține fișierul, cel mai nou primul, cu data, dimensiunea și destinația din care a fost citit. Un backup în care fișierul are aceeași dimensiune și aceeași oră de modificare ca în cel dinainte arată **neschimbat**, așa că vezi unde s-a schimbat fișierul ultima dată. Un clic pe un backup deschide fereastra de restaurare pe el, cu **Fișiere individuale** ales și fișierul bifat. Un fișier de pe unitatea flash revine ca parte din ZIP-ul unității flash, pentru că unitatea flash se restaurează doar în întregime.

Căutarea durează o vreme, pentru că restic trece prin fiecare backup. **Oprește** o încheie, la fel ca părăsirea paginii. Sub câmp scrie ce destinații a citit:

- Destinațiile de pe acest server și din rețeaua ta sunt căutate mereu.
- Un backup aflat în mai multe destinații este citit o singură dată, în prima. O destinație ale cărei backupuri au fost citite toate în altă parte arată **deja căutată în altă destinație**.
- Destinațiile externe, precum S3, B2 sau SFTP, rămân pe dinafară, dacă nu activezi **Caută și în destinațiile externe**. Atunci acolo sunt citite doar backupurile pe care nu le are nicio destinație de aici, de obicei cele mai vechi, pe care o retenție mai lungă le păstrează în afara locului. Prima căutare într-o astfel de destinație poate dura mult, iar unii furnizori taxează datele descărcate.
- O destinație care nu a putut fi deschisă arată **nu a putut fi citită**, iar (i) de lângă ea spune de ce. Celelalte destinații sunt căutate în continuare.

Mașinile virtuale sunt salvate ca discuri întregi, așa că fișierele din ele nu sunt căutate. Nici auto-backupul, nici backupurile unui alt BombVault de la **Alte surse** și nici cele ale serverelor de la **Alte servere** nu sunt căutate. O căutare listează primele 200 de fișiere, așa că pentru un nume scurt precum `log` poate fi nevoie de câteva litere în plus.

## Ce revine, pe tipuri {#kinds}

**Containerele** sunt recreate din definiția lor salvată prin API-ul Docker, cu entrypoint, hostname, setările de oprire, volume și montări, limite de resurse, driverul de log, setările DNS și legăturile vechi. Un container revine pe imaginea pe care rula când s-a făcut backupul, nu pe una mai nouă trasă ulterior, iar o imagine lipsă este trasă mai întâi, cu autentificarea din **Registre de containere** de la **Setări, Restaurare**. Revin și GPU-ul sau runtime-ul lui (`--gpus`, `--runtime=nvidia`). Pe o gazdă fără acel driver sau runtime, restaurarea spune asta și oferă **Restaurează fără GPU și runtime**. O legătură către un container care lipsește sau este oprit când cel restaurat pornește este omisă, iar istoricul rulărilor spune asta. O restaurare care eșuează după ce containerul a fost eliminat îl pune la loc, oprit.

Un backup poate conține foldere pe care containerul nu le mai are selectate, de exemplu unul mai vechi, făcut înainte să dispară un folder. Un folder care a dispărut sau este gol revine. Unul care conține acum fișiere rămâne cum este, pentru că s-ar putea să-l fi debifat intenționat, iar verificarea dinaintea restaurării și istoricul rulărilor îl numesc. Readu-l cu **Fișiere individuale** sau **Într-un folder**.

**VM-urile** își primesc din nou XML-ul definit prin SSH, iar discurile și NVRAM-ul UEFI sunt reatașate, chiar și după ce VM-ul a fost șters. Vezi [Backupuri VM](vm-backup-ssh-setup.md) pentru VM-urile cu instantanee Unraid și discuri pe zvol-uri.

**Seturile de foldere** revin cu proprietarul, permisiunile și marcajele de timp așa cum le stochează restic. Acolo unde ținta refuză doar proprietarul, cum face `/mnt/user`, rularea se încheie cu o notă care spune asta. BombVault compară apoi fișierele cu backupul, iar un fișier care nu a ajuns face restaurarea să eșueze.

**Unitatea flash** nu atinge niciodată `/boot` aflat în uz. Restaurarea este o descărcare numită `flash-<id>.zip`, gata de pus în Unraid USB Creator. ZIP-ul conține `APP_KEY`, așa că descărcarea lui cere o parolă de conectare și o autentificare chiar pe instanță. Fereastra listează și pluginurile din fiecare backup flash cu versiunea și dimensiunea lor și poate pune unul singur înapoi în unitatea flash aflată în uz: fișierul lui `.plg`, folderul lui de sub `config/plugins` și fișierele pachetului pe care le conține backupul. Unraid îl instalează la următoarea pornire sau pe loc, la **Plugins, Install Plugin**.

**Seturile de date ZFS** au ținte proprii și își păstrează proprietățile ZFS, vezi [Restaurare](zfs-datasets.md#restore).

**Auto-backupul** revine doar prin asistentul de urgență, care repornește BombVault, așa că baza de date aflată în uz nu este niciodată suprascrisă sub un handle deschis.

**Dumpurile bazelor de date** nu sunt niciodată reîncărcate singure. Un rând de la **Copii SQL ale bazei de date** poate fi descărcat, salvat într-un folder sau importat într-o bază de date tocmai pornită. Vezi [Dumpuri de baze de date](backups.md#database-dumps).

## Restaurarea mai multor elemente deodată {#several}

Pe pagina Restaurare alege elementele cum alegi rândurile din lista Copii de rezervă: Ctrl+clic, Shift+clic, Ctrl+A, o apăsare lungă sau **Selectează**. **Restaurare** deschide apoi o singură fereastră pentru toate.

- **Care copie de rezervă**: **Recomandată** ia pentru fiecare element backupul pe care l-ar recomanda propria lui fereastră. **Un singur moment pentru toate** ia pentru fiecare element cel mai nou backup dintr-un **Moment** ales de tine sau mai vechi, de exemplu seara dinainte ca ceva să meargă prost.
- **Unde**: **Lângă original** sau **Înlocuiește originalul**, cu **Fă mai întâi o copie a stării actuale**.

Fiecare element este verificat ca la o restaurare singulară, apoi rulează unul după altul. Fiecare rând arată cum i-a mers: **În așteptare**, **În rulare**, **Restaurat**, **Eșuat** sau **Lăsat afară**. **Oprește după acesta** lasă restaurarea care rulează să se termine și nu mai pornește altele. Unitatea flash vine ca un ZIP de descărcat, iar seturile de date ZFS și auto-backupul sunt restaurate pe rând.

![Restaurarea mai multor elemente: un rând pe element cu progresul lui.](assets/screenshots/restore/several-dark.png#only-dark)
![Restaurarea mai multor elemente: un rând pe element cu progresul lui.](assets/screenshots/restore/several-light.png#only-light)

## Restaurarea unui stack {#stacks}

Un clic pe un stack din cardul **Stack-uri** deschide **Restaurează stack-ul**. Restaurează toți membrii din ultimul lor backup în același timp, iar cu **Pornește în ordinea dependențelor după restaurare** îi pornește în ordinea `depends_on`: întâi bazele de date și cache-urile, apoi aplicația. Când folderul de proiect al unui membru lipsește de la locul ales, fereastra oferă în schimb starea de la Local.

BombVault verifică întâi membrii și folderul de proiect, iar o restaurare care nu ar putea reuși nu schimbă nimic. Altfel oprește membrii care rulează, aplicațiile înaintea bazelor lor de date, apoi scrie folderul de proiect, apoi restaurează membrii. Dacă folderul de proiect nu revine, membrii sunt totuși restaurați, dar rămân opriți, pentru că folderul poate fi scris doar pe jumătate și acolo stau de obicei bind mounturile unui membru. Rularea fiecărui membru spune asta.

O bază de date care are doar dumpuri de baze de date și niciun backup de fișiere, și al cărei container și volum numit au dispărut, revine fără datele ei. Verificarea spune asta înainte de restaurare. Rămâne oprită, la fel și containerele care depind de ea, ca nicio aplicație să nu își ruleze configurarea pe o bază de date goală. Pornește-o, importă un dump și pornește-le pe celelalte. Vezi [Dumpuri de baze de date](backups.md#database-dumps).

## Alte surse {#other-sources}

- **Dintr-un folder vechi de copii** deschide depozitul unui alt BombVault care nu mai rulează, pe un disc sau într-o destinație off-site, cu kitul lui de recuperare sau cu `APP_KEY` scris de tine. Sesiunea este doar pentru citire și trăiește doar în memorie. Vezi [Dintr-un folder vechi de copii](offsite-recovery.md#restore-from-another-bombvault-repo).
- **Import din pluginul Appdata.Backup** citește folderul de backup al pluginului. Fiecare arhivă de container devine un punct de restaurare al containerului ei, datat de când l-a făcut pluginul. Arhivele importate înainte sunt sărite, iar arhivele în sine sunt doar citite. Containerul are nevoie mai întâi de un backup în BombVault, ca restaurarea să aibă definiția lui. Retenția lasă în pace punctele de restaurare importate, așa că șterge-l singur pe unul când nu mai ai nevoie de el.
- **Sarcini din Duplicati sau Duplicacy** transformă sarcinile de backup ale acestor instrumente în seturi de foldere. Le citește doar setările. Backupurile vechi rămân la instrumentul vechi, iar BombVault începe cu un prim backup propriu. Niciunul dintre cele două instrumente nu trebuie să fie instalat aici.
    - **Din Duplicati**: în Duplicati, exportă fiecare sarcină într-un fișier fără parolă și alege fișierele `.json`. Un export care include parolele aduce cu el și autentificarea destinației.
    - **Din Duplicacy**: alege folderul Duplicacy Web care conține `duplicacy.json` sau un folder căruia îi faci backup cu linia de comandă Duplicacy, care conține `.duplicacy`. Cheile pe care linia de comandă le-a salvat în `.duplicacy/preferences` vin și ele. Duplicacy Web își păstrează autentificările criptate, așa că fereastra ți le cere.

    Nu se creează nimic înainte să vezi previzualizarea. Fiecare folder sursă devine un set de foldere, cu folderul de pe acest server unde a fost găsit, excluderile care au putut fi preluate și programul. Dacă instrumentul vechi rula într-un container propriu și un folder nu a fost găsit, îl alegi tu. Backupurile merg într-o destinație nouă, într-un folder `bombvault` din vechea destinație, fie că e un folder local, S3, SFTP, un remote rclone sau Backblaze B2 prin adresa lui S3. O autentificare din fișier se salvează împreună cu celelalte autentificări ale tale, iar cea care lipsește din fișier ți se cere. Notele de sub fiecare sarcină spun ce nu a fost preluat, cum ar fi regulile de includere, expresiile regulate, cât timp erau păstrate backupurile și parola veche de criptare, de care BombVault nu are nevoie.

## Alte servere {#other-servers}

Într-un [depozit comun](offsite-recovery.md#shared-repository) fiecare cheie deschide copiile tuturor serverelor, așa că pagina Restaurare îi listează pe ceilalți membri la **Alte servere**, fiecare cu numele, gazda, câte elemente are și când a rulat cea mai nouă copie a lui. O gazdă care nu aparține niciunui server cunoscut de acesta apare ca **Un server pe care acesta nu îl cunoaște**. Un clic deschide **Copii de rezervă de la** urmat de numele serverului, doar în citire și fără nimic de introdus. Seturile de foldere și seturile de date ZFS ajung într-un folder ales de tine. Containerele și VM-urile revin ca fișiere simple într-un folder, pentru că celălalt server le sigilează definițiile cu propria cheie. Copiile proprii ale acestui server din depozitul comun sunt în fila **Copii de rezervă** a fiecărui element.

## După o restaurare {#after}

O copie restaurată lângă original este listată la **Copii de rezervă** ca exclusă, deci nu contează ca sarcină deschisă. Adaugă-o în programare dacă vrei s-o păstrezi. Un container sau o VM care rula când s-a făcut backupul revine pornit, unul care era oprit rămâne oprit. Pe Unraid un container restaurat reapare în fila Docker ca o aplicație obișnuită, editabilă, iar o VM în VM Manager cu discurile și NVRAM-ul atașate.
