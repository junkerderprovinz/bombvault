# Off-site și recuperare

!!! note "Copiile off-site așteaptă după o reconstrucție"
    Când Asistentul de urgență sau **Descoperă copii de rezervă** reconstruiește elemente fără setările vechi, copiile off-site ale acelor secțiuni se opresc până când valoarea lor implicită este confirmată. Vezi [După o reconstruire](#after-a-rebuild).

Copiile locale te protejează de un container pierdut sau de o actualizare proastă. Copiile off-site și un kit de recuperare testat te protejează de pierderea întregii stații, de ransomware sau de un incendiu. Această pagină acoperă destinațiile și copiile off-site, cum faci o copie rezistentă la manipulare, cum dovedești că poți restaura, cum împerechezi mai multe servere și cum aduci totul înapoi când BombVault însuși a dispărut.

## Copii off-site {#off-site-copies}

Păstrează copia locală rapidă și adaugă una sau mai multe destinații în afara locuinței. BombVault copiază acolo copiile de rezervă noi cu `restic copy` pe bază de best-effort, așa că un sughiț off-site nu face niciodată să eșueze copia locală. În această formă depozitul local rămâne primar, iar cel off-site este o copie, dar depozitul primar al unei secțiuni nu trebuie să fie deloc local; vezi mai jos [Depozite primare la distanță](#remote-primary-repositories) pentru copierea direct în S3, un rest-server și altele asemenea, în loc să copiezi spre ele.

- **Mai multe destinații pe secțiune.** Fiecare secțiune (containere, VM-uri, flash, Auto-backup, foldere și seturi de date ZFS) poate copia simultan în mai multe destinații off-site, așa că poți ține, de exemplu, un rest-server pe stația unui prieten și un bucket S3 în paralel. Cardul **Destinații pe secțiuni** de pe pagina Destinații pornește sau oprește fiecare linie de la o secțiune la o destinație.
- **Când rulează copiile.** Cardul **Copiază** al unei destinații stabilește **Când se copiază**: **După fiecare copie de rezervă** sau după un plan propriu, care adună copiile de rezervă și copiază la ore fixe. La **Setări, Programare, Copii externe pe secțiuni**, **Excepție** dă unei secțiuni un plan propriu de copiere. **Replică acum** în cardul de copie off-site al unei secțiuni de la **Setări, Destinații** pornește o copiere la cerere.
- **Retenția** se stabilește pe fiecare destinație la **Cât timp se păstrează**, așa că poți ține copiile off-site mai mult, ca arhivă. Copia din câmpul off-site al unei secțiuni urmează cardul **Retenție externă** de la **Setări, Destinații**; las-o toată pe zero ca să nu tai niciodată automat copiile de rezervă off-site.
- **Limitele de lățime de bandă** (**Lățime de bandă externă** la **Setări, Destinații**, sau cardul **Copiază** al unei destinații) plafonează rata de upload/download restic ca să nu satureze copiile WAN-ul tău.
- Cât timp rulează o copiere, antetul de stare din prezentare spune **Copie off-site în curs**. Este un indicator de activitate, nu o bară de procente, pentru că `restic copy` nu expune progres citibil automat.

!!! note "Restaurează din orice loc"
    Fiecare element își listează copiile ca o singură axă a timpului peste toate locurile în care se află o copie, în fila lui **Copii de rezervă**. O copie trimisă la B2 apare o dată, marcată cu fiecare loc care o deține. O restaurare ia primul loc pe care îl poate atinge, începând cu depozitul în care este scris elementul, și poți alege alt loc per copie de rezervă. Destinațiile off-site sunt citite doar când apeși **Verifică**, pentru că listarea stocării cloud durează. Ștergerea dintr-un loc verifică întâi celelalte și spune dacă era ultima copie.

## Destinații {#destinations}

O destinație este un loc unde ajung copiile de rezervă: un folder local, o partajare, un server al tău sau un serviciu cloud. Se configurează o singură dată și servește apoi fiecare secțiune. Deschide **Destinații** din placa ei de setări sau pornește acolo **Arată și în bara laterală** ca să o fixezi.

**Adaugă destinație** deschide o fereastră în patru pași:

1. **Furnizor.** Fiecare serviciu este listat cu logoul lui, cu o căutare deasupra, pe grupe: servicii de stocare cu bucket-uri S3 (Backblaze B2, Wasabi, Cloudflare R2, Hetzner Object Storage, Amazon S3 și altele), propriul tău server S3 (Garage, SeaweedFS, RustFS, Silo, Ceph, JuiceFS, Versity S3 Gateway), propriul tău server și partajări (rest-server, Hetzner Storage Box, SFTP, SMB, WebDAV, o cale montată), stocare cloud (OneDrive, Google Drive, Dropbox, pCloud, Nextcloud și restul pe care le acceptă rclone) și serverele receptoare ale grupului tău. Fiecare spune cât de bine se potrivește pentru copii de rezervă: unitățile cloud încetinesc la multe cereri, așa că prima copie și curățarea durează mai mult acolo.
2. **Autentificare.** Câmpurile depind de serviciu: o cheie de acces pentru S3, un utilizator și o parolă pentru WebDAV și SMB, o parolă de aplicație acolo unde autentificarea în doi pași o blochează pe cea obișnuită, cheia publică SSH a BombVault pentru SFTP și Storage Box sau un token pentru serviciile care se autentifică printr-un browser. Pentru acestea, fereastra arată o comandă `rclone authorize` de rulat pe un calculator cu browser; tokenul pe care îl tipărește merge în câmp. **Testează** verifică autentificarea înainte să se salveze ceva. Fereastra listează apoi folderele de pe destinație, cu **Folder nou** pentru a crea unul și cu spațiul liber acolo unde serviciul îl raportează. Un folder gol este cel mai sigur; BombVault pune în el câte un folder pe secțiune.
3. **Protecție.** Fereastra spune clar ce poate face serviciul. Un rest-server în mod append-only refuză ștergerea, iar testul de manipulare verifică asta. Un bucket S3 poate păstra versiuni vechi prin versionare și object lock, ceea ce BombVault nu poate verifica încă. O unitate cloud nu poate refuza deloc ștergerea: cine ajunge în server ajunge și la acea copie. Pornește **Protecție împotriva ștergerii (append-only)** doar acolo unde partea îndepărtată chiar refuză ștergerea; BombVault nu mai curăță atunci niciodată acolo. **Pentru urgențe** spune ce conține kitul de recuperare pentru această destinație.
4. **Pentru ce secțiuni.** Alege secțiunile care se salvează aici. Prima copiere aduce tot istoricul. Elementele care și-au ales propriile destinații nu o primesc singure pe cea nouă, iar fereastra le numește.

Serviciile S3 rulează prin backendul S3 propriu al restic, ceea ce permite aplicarea unei clase de stocare și a object lock. Orice alt serviciu rulează prin rclone-ul livrat cu BombVault, iar remote-ul lui apare apoi în cardul **În afara sitului (rclone)** de la **Setări, Destinații**. Un export de setări poartă destinațiile; cu credențialele incluse poartă și autentificarea lor.

Folderul de pe acest server se numește **Local** peste tot: pe paginile destinațiilor, în flux, la un element și la Restaurare. Dacă pe acest server există mai multe foldere, fiecare poartă numele lui propriu. O destinație pe care o creează câmpul off-site al unei secțiuni se numește după locul ei, adică ultimul folder din adresa ei sau, dacă nu are unul, după serverul ei, și primește un număr când o altă destinație are deja acest nume. Un nume pe care i-l dai tu pe pagina ei rămâne. Când două destinații cu același nume stau una lângă alta, fiecare poartă și serverul sau folderul ei.

Fiecare destinație are o pagină proprie:

- **Conexiune** cu numele, comutatorul **Disponibil** (oprit înseamnă că BombVault nu mai scrie aici, iar elementele care se salvează aici eșuează cu un mesaj clar în loc să ajungă în altă parte), folderul, compresia, clasa de stocare și **În afara sediului**. **Schimbă conexiunea** testează și salvează o adresă sau o autentificare nouă.
- **Credențiale**: setul partajat sau unul propriu.
- **Protecție împotriva ștergerii**, cu **Testează acum protecția împotriva ștergerii** și cine curăță această copie.
- **Copiază**: când se copiază și limitele de upload și download.
- **Cât timp se păstrează**: **Scurt**, **Echilibrat**, **Lung** sau **Valori proprii**, cu o axă a timpului a copiilor pe care le păstrează regula. **Întreabă înainte de a păstra mai puțin** întreabă înainte ca o regulă mai scurtă să șteargă copii al căror unic exemplar este aici.
- **Reguli pe secțiuni**: depozitul, retenția sau compresia proprie a unei secțiuni la această destinație. **Schimbă regulile** le deschide, iar **Revino la destinație** readuce o secțiune.
- **Deja sub această destinație**: secțiuni a căror adresă tastată manual se află într-un folder al acestei destinații. Preluarea uneia lasă depozitul unde este; autentificarea, clasa de stocare și protecția la ștergere vin apoi de la destinație. BombVault verifică întâi dacă autentificarea destinației deschide depozitul și refuză să pună un depozit append-only sub o destinație care nu este append-only.
- Un vas care arată cât de plină este destinația, cu marcajul unde se umple și câte săptămâni mai sunt la ritmul de creștere din ultimele săptămâni. O destinație care nu își raportează dimensiunea spune **Dimensiune necunoscută**.

Un server receptor pe care îl rulează altă instanță a grupului tău apare printre furnizori ca **Din grupul tău**; vezi [Server receptor](#receiving-server).

### Mutare și eliminare {#move-remove}

- **Mută** duce copia unei secțiuni la altă destinație, cu tot istoricul ei. Totul rămâne în locul vechi; BombVault nu șterge nimic acolo. Mutarea rulează în fundal și poate dura mult: aproximativ 9 minute la 100 GB între discuri locale, ore pe o legătură lentă. Prezentarea o arată cât timp rulează.
- **Elimină destinația** oferă două căi. **Elimină destinația, păstrează copiile** nu mai scrie acolo și lasă copiile pe loc; le poți citi mai târziu la **Restaurare, Dintr-un folder vechi de copii**. **Elimină destinația și șterge copiile de acolo** citește întâi ce este stocat acolo și îl listează: ce copii dispar, care dintre ele nu mai sunt nicăieri altundeva și ce depozite rămân pentru că protecția la ștergere refuză, elemente încă se salvează direct în ele, altă destinație sau secțiune își păstrează copiile acolo ori nu au putut fi citite. Elementele care și-ar pierde unica copie sunt listate separat și sunt păstrate dacă nu bifezi **Șterge și acestea**. Confirmi tastând numele destinației, iar ștergerea rulează în fundal.
- Ștergerea copiilor de la o destinație cere o parolă de conectare și o autentificare chiar pe instanță. Fără parolă destinația poate fi totuși eliminată, doar copiile ei nu pot fi șterse de aici.

## Amplasare per element {#placement}

Cardul **Programare și destinație** din fila Setări a unui element are un rând de butoane: **Local** și câte un buton pentru fiecare destinație off-site a secțiunii. Butoanele aprinse primesc copiile elementului.

- Cu **Local** aprins, elementul este scris în depozitul arătat la **Stocat pe** și copiat în fiecare altă destinație aprinsă. Întunecă o destinație și nu mai primește nimic nou de la acest element. Doar Local nu copiază nicăieri, ceea ce se potrivește datelor care au deja o a doua copie, de exemplu o partajare aflată pe un NAS.
- Cu **Local** întunecat, elementul este scris direct în depozitul direct al primei destinații aprinse și copiat de acolo în celelalte aprinse. Prima dată, o fereastră creează acel depozit direct.
- Un buton rămâne aprins, pentru că o copie are nevoie de un loc unde să meargă. Ca să lași un element în afara copiilor, folosește **Fără copie de rezervă**.

BombVault nu mută niciodată copii între depozite. Un element care urmează secțiunea lui o urmează și când valoarea implicită a secțiunii se schimbă: următoarea lui copie ajunge în locul nou, iar copiile pe care le are rămân unde sunt. Un loc ales chiar pe element rămâne fixat din prima lui copie; cardul **Depozit** din fila Setări îl numește. Copiile se pot schimba oricând. O destinație care nu mai primește un element își păstrează copiile pe care le are și le taie la retenția ei la următoarea rulare off-site a secțiunii. Din destinațiile append-only nu se poate șterge.

Locul pe care un element l-a părăsit își păstrează copiile. În fila **Copii de rezervă** a elementului și la **Restaurare** apare ca **Doar copii de rezervă mai vechi**, o restaurare poate citi în continuare de acolo, iar o curățare de acolo lasă în pace aceste copii.

**Urmează setarea implicită** readuce un element cu alegere proprie la secțiunea lui. Întreabă întâi, spune unde merg atunci copiile noi și numește ce nu mai primește fiecare destinație și ce ar încărca următoarea rulare.

Sub rând, cardul spune unde merge elementul și ce există efectiv: câte locuri îl păstrează, când a fost văzută ultima oară fiecare destinație și dacă 3-2-1 este îndeplinit. Un loc este serverul cu datele originale, fiecare destinație off-site și fiecare depozit marcat **În afara sediului**. BombVault verifică copiile și locurile; nu verifică partea „două suporturi” din 3-2-1.

### Valori implicite pe secțiuni {#placement-defaults}

Cardul **Destinații pe secțiuni** de pe pagina Destinații desenează valoarea implicită a fiecărei secțiuni ca un flux 3-2-1. Fiecare linie de la o secțiune la o destinație este un comutator, **Local** inclus, pentru fiecare secțiune, deci și pentru unitatea flash, seturile de date ZFS și auto-backup. Un clic pe o linie sau pe un punct o comută imediat, fără întrebare; liniile punctate sunt oprite. **Grafic / Comutatoare** arată același lucru ca rânduri de butoane. Un comutator hotărăște unde merg copiile noi. Elementele fără alegere proprie și folderele de proiect ale stack-urilor Compose urmează imediat, deci următoarea lor copie merge la locurile noi. Copiile deja stocate rămân unde sunt și pot fi restaurate în continuare de acolo.

Cu **Local** oprit și doar destinații off-site pornite, o secțiune face copia direct în ele, iar BombVault alege singur depozitul de acolo. Containerele, VM-urile și seturile de foldere merg în depozitul direct al primei destinații pornite, descris mai jos, și sunt copiate de acolo în celelalte. Unitatea flash, seturile de date ZFS și auto-backupul scriu fiecare copie în fiecare destinație pornită, iar fiecare destinație le păstrează după retenția ei proprie.

O secțiune cu toate liniile oprite spune **Fără copie**. Elementele ei nu primesc copii și contează ca neprotejate.

Elementele care și-au ales propriile destinații pe pagina lor le păstrează. Fluxul spune câte sunt într-o secțiune, de exemplu **2 intrări diferă**, și duce la ele: la fila Setări a elementului când este unul, la o listă cu locul unde se salvează fiecare când sunt mai multe. **Urmează setarea implicită** de acolo readuce un element.

O destinație off-site nouă primește fiecare element care nu este setat pe Local. Fereastra care o adaugă spune câte elemente și, unde se știe, cât istoric înseamnă asta.

### Depozite directe

Un depozit direct este locul unde elementele fac copia direct pe o destinație off-site. BombVault îl pune lângă folderul destinației, cu `-direct` adăugat la nume sau, când destinația stă la rădăcina bucket-ului, a remote-ului sau a unui utilizator rest-server, într-un folder din ea numit după secțiune, de exemplu `containers-direct`. În fluxul de pe pagina Destinații este creat fără întrebare. Oprirea lui Local pentru un singur element, astfel încât o destinație fără depozit direct să devină locul lui, deschide o fereastră cu locația sugerată, de exemplu `s3:https://s3.eu-central-003.backblazeb2.com/bucket/containers-direct`, și un test de conexiune care nu creează nimic. **Creează și folosește** creează depozitul și îndreaptă elementul spre el. Un depozit direct preia cheia destinației, clasa de stocare, limitele, setarea append-only și retenția și se schimbă odată cu ele; cardul Depozite îl arată doar în citire. Când o cheie nouă a destinației nu îl poate deschide, depozitul direct își păstrează cheia pe care o are, iar salvarea spune asta. Un element dintr-un depozit direct este copiat de acolo în celelalte destinații aprinse, niciodată în destinația căreia îi aparține depozitul. Copiile lui de rezervă poartă eticheta `bv:direct`, iar orice altă trecere de retenție le păstrează, deci un depozit direct care și-a pierdut legătura cu destinația lui nu îmbătrânește niciodată după regulile locale. B2 se atinge prin punctul său final S3, cu ID-ul cheii și cheia aplicației introduse ca acreditări S3; o cheie limitată la folderul propriu al destinației nu poate ajunge în folderul de lângă el, așa că limitează cheia la folderul de deasupra.

### În afara sediului

O destinație sau un depozit denumit poate fi marcat **În afara sediului**. Cele la distanță pornesc marcate; oprește marcajul pentru un rest-server din aceeași clădire. Marcajul doar numără locurile și 3-2-1 pe carduri. Nu schimbă nicio copie.

O replică ZFS este o copie în afara sediului, dar nu o copie de rezervă cu istoric. Păstrează doar instantaneele pe care le stabilești, iar versiunile vechi și fișierele individuale vin tot din copiile de rezervă. Merge de pe un server ZFS direct pe celălalt, prin SSH către un server ZFS sau către punctul de primire al unei instanțe împerecheate. Grupul de împerechere poartă doar cererea. Vezi [Replică](zfs-datasets.md#replica).

### După o reconstruire {#after-a-rebuild}

Alegerile de copiere stau în setările proprii ale BombVault. După o reconstruire prin **Descoperă copii de rezervă** sau prin Asistentul de urgență fără setări restaurate, ele dispar, iar copierea a tot ar trimite din nou la B2 elementele pe care le lăsaseși deoparte. Copiile off-site ale fiecărei secțiuni reconstruite se opresc de aceea. Prezentarea spune asta, iar cardul **Destinații pe secțiuni** oferă **Confirmă valoarea implicită** cu o previzualizare a ce copiază rularea următoare și numele din copiile fără element, pe care le poți lăsa deoparte acolo. Doar confirmarea încheie pauza; importul unui fișier de setări readuce regulile și valorile implicite, dar nu o încheie.

## Depozite primare la distanță {#remote-primary-repositories}

Calea proprie a unei secțiuni nu se limitează la un folder local: îndreapt-o direct spre un remote restic (`s3:...`, `rest:http://host:8000/repo`, `sftp:user@host:/repo`, `rclone:remote:bucket/path`) și BombVault face copia direct acolo, fără o copie locală separată și fără pas de copiere. Aceasta este o altă formă decât copiile off-site de mai sus: acolo depozitul local este primar, iar cel off-site este o arhivă best-effort a lui; aici depozitul la distanță **este** cel primar și este unica copie, afară de cazul în care copiezi acea secțiune și într-o altă destinație.

Calea fiecărei secțiuni (Containere, VM-uri, Flash, Auto-backup, Foldere, Seturi de date ZFS) se stabilește pe pagina destinației **Local**: **Schimbă** de lângă secțiune, în cardul ei **Conexiune**, deschide o fereastră cu un comutator **Local / La distanță**:

- **Local** arată navigatorul de foldere obișnuit.
- **La distanță** îl înlocuiește cu un câmp URL simplu și cu setările de conexiune pe care le folosește o destinație off-site, configurate aici pentru acest depozit primar. De acolo primești:
    - **Un test de conexiune** pe calea activă, înainte să te bazezi pe ea.
    - **Limite de lățime de bandă** (upload/download) ca o copie programată către un depozit primar la distanță să nu satureze WAN-ul tău: aceleași opțiuni restic `--limit-upload`/`--limit-download` pe care le folosesc copiile off-site, aplicate copierii însăși.
    - **Protecție append-only**, verificată cu același test activ de manipulare (o sondă DELETE reală către partea îndepărtată) pe care îl primesc destinațiile off-site. Cu ea pornită, BombVault refuză să curețe el însuși depozitul, pentru că în spatele lui nu există o copie locală separată, iar credențialele de pe această stație nu trebuie să poată șterge unica copie.
    - **O alarmă pentru bugetul de creștere**, eșantionată din aceeași tendință a dimensiunii depozitului pe care o urmărește deja cardul Stocare.

Nimic din acestea nu este obligatoriu: o cale la distanță tastată manual, fără setări de siguranță salvate, face copii nelimitate, curățabile și fără alarmă de buget. Setările de siguranță există pentru când vrei protecțiile pe care le primește o copie off-site, fără o destinație separată doar pentru ele.

!!! note "Credențialele cloud/REST sunt partajate"
    Un depozit primar la distanță se autentifică cu credențialele cloud partajate de la **Setări, Destinații**; nu există un depozit de credențiale separat pentru depozitele primare.

### SMB și WebDAV fără montare pe gazdă {#smb-webdav}

Cardul **În afara sitului (rclone)** de la **Setări, Destinații** are un formular pentru o partajare Windows sau Samba și pentru un server WebDAV (Nextcloud, ownCloud, SharePoint sau oricare altul). Completează un nume scurt, gazda și partajarea (SMB) sau URL-ul și tipul serverului (WebDAV), utilizatorul și parola, iar BombVault scrie secțiunea rclone pentru tine. rclone ofuscă parola singur înainte să fie stocată; adăugarea uneia cu un nume care există deja înlocuiește acea secțiune în loc să adauge una a doua. **Adaugă destinație** oferă SMB și WebDAV ca furnizori, cu aceleași câmpuri.

Formularul răspunde cu destinația gata făcută, de exemplu `rclone:nas:backups`. Folosește-o drept cale a unei secțiuni sau a unei destinații și adaugă un subfolder dacă vrei unul (`rclone:nas:backups/bombvault`). Partajarea este primul segment al căii, nu face parte din nume.

Aceasta este calea mai bună decât montarea partajării pe Unraid: restic nu recomandă păstrarea unui depozit pe o partajare CIFS montată, iar aici nu se montează nimic. NFS nu este în formular pentru că nici restic, nici rclone nu au un backend NFS; pentru NFS, montează exportul pe gazdă și folosește-l ca folder local.

## Protecție împotriva ștergerii (append-only)

Pornește protecția împotriva ștergerii pentru o destinație off-site ca ransomware-ul sau o gazdă compromisă să nu poată șterge sau rescrie copiile tale. Partea îndepărtată (un `restic/rest-server` care rulează în modul `--append-only`) o **impune**. BombVault doar o **verifică** și nu arată niciodată verde pe baza unei simple afirmații de configurare. Un folder de pe acest server nu poate refuza ștergerea, pentru că cine controlează BombVault controlează și folderul, așa că destinațiile locale nu oferă comutatorul.

Cea mai rapidă cale spre o copie append-only este un BombVault împerecheat cu un [server receptor](#receiving-server): el rulează rest-server-ul pentru tine, în mod append-only.

!!! note "O ștergere reușită sub `/locks/` este de așteptat"
    Append-only nu înseamnă că nu se poate elimina niciodată nimic. restic trebuie să ia și să elibereze propriile lock-uri, așa că `/locks/` rămâne scriibil și ștergibil prin concepție. Instantaneele și datele din spatele lor, adică ținta ransomware-ului, nu pot fi eliminate. Dacă sondezi tu însuți partea îndepărtată, o ștergere reușită sub `/locks/` este comportament corect și nu o breșă în protecție.

!!! warning "Depozitele append-only nu sunt niciodată curățate de pe această stație"
    O destinație append-only nu curăță niciodată de aici copiile de rezervă vechi; curățarea este treaba părții îndepărtate. Setează pentru ea o **alarmă pentru bugetul de creștere** ca să fii alertat înainte ca dimensiunea depozitului să scape de sub control.

## Test de manipulare

BombVault dovedește periodic garanția append-only încercând efectiv o ștergere în depozitul off-site, țintită la un obiect inexistent. **Testează acum protecția împotriva ștergerii** pe pagina unei destinații îl rulează la cerere, iar **Verifică protecția la ștergere** la **Setări, Verificare, Verificări automate** îl rulează după un program.

- **Refuzat** înseamnă protejat.
- **Acceptat** înseamnă neprotejat.
- Un rezultat **neconcludent** (server inaccesibil, eroare de autentificare) nu schimbă niciodată verdictul stocat.

O trecere reală de la protejat la neprotejat declanșează o singură alertă.

## Exerciții DR

BombVault oferă două niveluri de dovadă că copiile tale chiar se pot restaura, nu doar că există.

- **Exerciții de verificare a restaurării (local).** BombVault rulează periodic `restic check --read-data-subset` (limitat, niciodată o restaurare completă care umple discul). Cadența se află la **Setări, Verificare, Verificări automate**, iar cardul **Verifică copiile de rezervă** de pe aceeași placă arată rezultatul.
- **Restaurare de probă (off-site).** BombVault restaurează o țintă reală din depozitul off-site într-un sandbox izolat, o verifică fișier cu fișier și octet cu octet, apoi curăță. Asta dovedește că poți recupera din off-site, nu doar că depozitul răspunde.

Cardul **Stare protecție** din prezentare rezumă asta pe secțiuni în patru verificări: o copie actuală pe acest server, o copie off-site, protecție la ștergere acolo și o restaurare de probă de acolo. Un clic pe o secțiune arată detaliile, iar cardul devine verde doar pe fapte verificate.

## Împerecherea instanțelor {#pairing}

Mai multe BombVault-uri lucrează împreună ca un grup: își arată reciproc starea de protecție, preiau roluri unul pentru altul și își pot păstra reciproc cheia. O instanță se alătură grupului cu douăsprezece cuvinte.

Pe prima instanță deschide **Setări, Împerechere** și apasă **Generează frază**. Trei carduri de pași arată cum merge. Douăsprezece cuvinte apar într-o fereastră cu un buton **Copiază**. Pe fiecare altă instanță deschide același loc, apasă **Introdu frază** și lipește sau tastează cuvintele, sau apasă **Lipește** în acea fereastră. Un cuvânt care nu este în listă este numit cu poziția lui pe măsură ce tastezi, iar ultimul cuvânt poartă o sumă de control, așa că un cuvânt greșit sau schimbat este prins înainte să se împerecheze ceva. Generează fraza pe o singură instanță: două instanțe care generează fiecare una formează două grupuri separate. Dacă nu apare nimeni timp de un minut, pagina oferă două ieșiri: arată din nou cuvintele ca să le introduci dincolo, sau introdu cuvintele celeilalte instanțe și intră în grupul ei dintr-un singur pas. Împerecherea funcționează fără o parolă de conectare, dar setează una: fără ea oricine poate deschide interfața web poate citi cuvintele și, prin grup, poate cere roluri care dau mai departe parola restic a instanțelor din el. Cardul de împerechere spune asta până se setează o parolă. Cu o parolă, afișarea din nou a frazei o cere. **Părăsește grupul** scoate din nou o instanță.

Oricine cunoaște cuvintele poate intra în grup, așa că tratează-le ca pe o parolă.

**Cum se ajung membrii.** Fiecare instanță își află propria adresă din rețea de la browserul tău în clipa în care te autentifici, listată la **Adrese cunoscute** pe pagina de împerechere împreună cu cele găsite de instanță însăși; corecteaz-o acolo dacă în față stă un reverse proxy sau un port neobișnuit. În aceeași rețea membrii anunță acea adresă prin multicast și vorbesc direct, iar acolo unde multicastul nu poate trece printr-o rețea de containere, cum este puntea implicită Docker, o instanță își caută în schimb propria subrețea pentru ceilalți cu un apel semnat la care poate răspunde doar un membru al grupului, așa că împerecherea se încheie tot în câteva secunde fără relay. Dacă nu apare nimic, **Nu o găsești?** primește o adresă introdusă manual, pentru altă subrețea sau un port neobișnuit. Instanțele din rețele diferite trec printr-un relay, ales pe aceeași pagină; cele patru forme ale lui vin fiecare cu o imagine:

- **Relay-ul proiectului** (implicit): `parleyport.halleluja.design`, relay-ul pe care îl folosește și KnightLoader. Nu ai nimic de configurat.
- **Relay propriu**: containerul [**ParleyPort**](https://github.com/junkerderprovinz/parleyport) din Unraid Community Apps (și la **Setări, Aplicații**) sau una dintre instanțele tale care este deja accesibilă din exterior, cu **Servește ca relay** pornit. Acea instanță răspunde atunci la `/relay/connect` pe propria adresă, în spatele reverse proxy-ului și certificatului pe care le are deja, și lasă să intre doar grupul tău. Introdu adresa relay-ului pe fiecare instanță care ar trebui să-l folosească.
- **Fără relay**: membrii se găsesc automat în aceeași rețea și nicăieri altundeva.

**Ce vede relay-ul.** Fiecare apel între membri este sigilat cu AES-256-GCM sub o cheie derivată din cele douăsprezece cuvinte, iar acea cheie nu părăsește niciodată instanțele tale. Relay-ul află un hash care grupează conexiunile, pentru ce instanță este un mesaj, cât de mare e și când trece. Un apel direct în rețeaua locală este sigilat la fel și semnat în plus, așa că nimic nu depinde de certificatul autosemnat pe care îl servește o instanță. Apelurile care dau ceva mai departe (răspunsuri la roluri, o parolă restic, autentificări ale receptorului, depuneri de chei) sunt semnate și cu o cheie a membrului care le trimite, fixată la prima vedere; vezi [Model de securitate](configuration.md#security-model).

**Ce circulă prin grup.** Starea de protecție de pe pagina Instanțe, o cerere de a verifica acum o secțiune, cererile de roluri și răspunsurile lor, ofertele de stocare și ce are nevoie un receptor sau un preluator odată ce rolul i-a fost permis: locațiile depozitelor celeilalte instanțe și parola ei restic. Datele de copie nu circulă niciodată; merg tot direct la backendurile restic. Nici APP_KEY: parola restic deschide depozitele acelei instanțe și nimic altceva, nu secretele ei stocate, sesiunile sau codurile de recuperare.

**Intrări de dinainte de împerechere.** Instanțele adăugate cu un token de flotă și depozitele primite și sursele de preluare configurate cu APP_KEY-ul celeilalte instanțe rămân după actualizare și sunt marcate **Împerechează din nou**. Continuă să funcționeze: la prima pornire, BombVault înlocuiește fiecare APP_KEY stocat cu parola restic derivată din el. Împerechează ambele instanțe, apoi editează intrarea și alege instanța ei. Până atunci astfel de intrări stau în cardul **Fără instanță** de pe pagina Instanțe.

Singurul loc care mai primește un APP_KEY introdus manual este [Dintr-un folder vechi de copii](#restore-from-another-bombvault-repo), pentru cazul în care cealaltă instanță a dispărut și nu poate răspunde într-un grup.

## Instanțe și roluri {#roles}

Pagina **Instanțe** (o placă de setări care poate fi fixată în bara laterală) arată grupul ca o rețea cu cine ce face pentru cine, un card pentru această instanță și un card pentru fiecare alt membru, telefoanele Android și serverele ZFS. **Împerechere** plutește jos în dreapta. Ce părți sunt pornite se stabilește la **Setări, Instanțe, Mai multe servere**: **Instanțe**, **Receptor** și **Preluare**.

Fiecare card arată starea de protecție a membrului și rolurile lui ca plăci. Un rol este ceva ce o instanță face pentru alta:

- **Receptor** păstrează copiile off-site ale acestui server într-un rest-server în mod append-only. Acest server trimite după fiecare copie și nu poate șterge nimic acolo, chiar dacă îl preia cineva.
- **Preluator** preia copiile acestui server după un program propriu. Acest server nu trebuie să fie treaz pentru asta.
- **Server ZFS** preia replicile ZFS ale acestui server. Fiecare element ZFS îl alege la **Replică** în setările lui.
- **Păstrător de cheie** păstrează o copie sigilată a kitului de recuperare al acestui server. Vezi [Păstrătorul de cheie](#key-keeper).

Pornirea unui rol trimite o cerere, și nimic nu începe până când cineva de pe cealaltă instanță apasă **Permite**. Cererea apare pe cardul acelei instanțe ca insignă, iar **Detalii** o listează cu **Permite** și **Refuză**; un rol permis poate fi revocat acolo oricând. O linie punctată în rețea este un rol care încă așteaptă. O instanță cu BombVault 9.x nu poate răspunde la o cerere, deci un rol este activ acolo imediat ce îl pornești, iar o cerere de la o instanță 9.x așteaptă aici **Permite**.

**Detalii** pe un card deschide restul: depozitele primite și sursele de preluare ale acelui membru, ofertele de stocare, monitorizarea, **Elimină** și **Uită cheia**.

## Receptor și preluator {#receiver}

![O copie off-site primită, în detaliile unei instanțe, urmărită doar în citire, cu o verificare de integritate rulată pe acest hardware.](assets/screenshots/receiver.png)

*O copie off-site primită, în detaliile unei instanțe, urmărită doar în citire, cu o verificare de integritate rulată pe acest hardware.*

Pe stația care **primește** copii append-only de la alt BombVault, depozitele primite îți oferă monitorizare independentă, doar în citire, pe hardware-ul care primește, ca un eșec silențios de la capătul îndepărtat să nu treacă neobservat. Pornește **Receptor** la **Setări, Instanțe**, apoi deschide **Detalii** ale instanței care trimite, pe pagina Instanțe, și adaugă depozitul primit (doar în citire, deschis cu parola restic a instanței care trimite, pe care o primește prin [grupul de împerechere](#pairing)). Primești:

- **Un inventar pe surse**, ca să vezi exact ce containere, VM-uri și seturi de foldere au ajuns.
- **Ultima primire** pe sursă, ca să știi cât de proaspătă este fiecare.
- **Un `restic check` independent** rulat pe hardware-ul care primește, ca integritatea să fie verificată acolo unde se află datele, nu doar la expeditor.
- **Un comutator de om mort:** o alertă când o sursă nu mai trimite într-o fereastră stabilită de tine.
- **Alerte de integritate:** o alertă când o verificare de pe partea care primește eșuează.

Receptorul este strict doar în citire. Nu scrie niciodată în depozitul primit, deci nu poate strica niciodată garanția append-only pe care se bazează expeditorul.

Un preluator lucrează invers: cu **Preluare** pornită, o instanță citește copiile altui membru în propriul depozit după un program propriu. Sursele lui stau și ele în detaliile acelui membru.

### Server receptor {#receiving-server}

Stația care primește poate rula și rest-server-ul spre care copiază ceilalți. Se configurează prima dată când permiți o cerere **Receptor** care îl cere: fereastra spune că serverul are nevoie de un server receptor și cere un folder pe o partajare, cu **Dosar nou** pentru a crea unul, și un port (8000, dacă nu-l folosește alt container). BombVault apoi:

1. refuză dacă există deja un container numit `rest-server` sau dacă alt container ține portul;
2. trage `restic/rest-server` și îl pornește prin socketul Docker în mod append-only, cu depozite private și un fișier de autentificare în folder;
3. scrie șablonul lui Unraid pe unitatea flash, ca containerul să rămână editabil în fila Docker, sau oferă șablonul ca descărcare când unitatea flash nu poate fi atinsă;
4. rulează testul de manipulare împotriva lui și arată dacă refuză ștergerile.

Instanțele din grupul tău găsesc apoi serverul printre furnizori la **Din grupul tău**, numit după stația care primește, iar rolul **Receptor** îl oferă ca alegerea sigură. Fiecare instanță primește o autentificare proprie prima dată când folosește serverul și scrie doar în propriul folder de acolo. Cu **Receptor** pornit la **Setări, Instanțe**, **Server receptor** pe cardul acestei instanțe redeschide serverul. Fereastra lui listează autentificările la **Parteneri cu autentificare**, iar **Revocă autentificarea** o scoate pe una; ce a copiat deja acea instanță rămâne în folder. Configurarea creează și o **Autentificare din afara grupului**, a cărei parolă fereastra o arată o singură dată.

O instanță care ajunge la stația receptoare doar prin relay nu poate folosi serverul, pentru că relay-ul nu transportă copii. Adaugă mai întâi adresa stației receptoare pe pagina de împerechere. Când BombVault rulează pe o adresă IP proprie (pe br0, de exemplu), completează **Adresă pentru parteneri**, pentru că serverul ascultă pe adresa gazdei.

## Exemplu complet: două stații Unraid, de la un capăt la altul

Tot ce e mai sus descrie piesele. Aceasta este o configurare completă, pentru că piesele se asamblează mai ușor după ce le-ai văzut asamblate o dată.

Două stații: **TOWER** rulează containerele și trimite copiile; **VAULT** le primește și impune append-only. Înlocuiește cu propriile nume, adrese și căi de partajare.

**1. Împerechează cele două stații.** Pe TOWER deschide **Setări, Împerechere**, apasă **Generează frază** și introdu cele douăsprezece cuvinte pe VAULT la **Introdu frază**. Setează mai întâi o parolă de conectare pe amândouă.

**2. Pe TOWER, cere ca VAULT să fie receptor.** Pe pagina Instanțe, pornește **Receptor** pe cardul VAULT, alege secțiunile și **serverul receptor al VAULT** ca loc și trimite cererea.

**3. Pe VAULT, permite.** Pagina Instanțe a VAULT arată cererea pe cardul TOWER. Apasă **Permite**. Prima dată, VAULT cere un folder pentru serverul lui receptor, de exemplu `user/appdata/rest-server`, pornește rest-server-ul în mod append-only și rulează testul de manipulare împotriva lui.

**4. Înapoi pe TOWER.** De atunci TOWER copiază spre VAULT după fiecare copie de rezervă, iar noua destinație apare pe pagina Destinații a TOWER cu protecția ei la ștergere.

Ce răspunde testul de manipulare:

| Rezultat | Ce s-a întâmplat |
| --- | --- |
| **protejat** | VAULT a refuzat ștergerea. Aceasta este singura stare de succes. |
| **neprotejat** | VAULT a acceptat o ștergere. `--append-only` lipsește sau a fost scos. |
| **neconcludent** | Niciuna. De obicei URL-ul nu este cel pe care îl folosește restic însuși sau credențialele s-au schimbat. Nu se înregistrează nimic și nu pornește nicio alertă. |

**5. Urmărește ce sosește, dacă vrei.** Pornește **Receptor** la **Setări, Instanțe** pe VAULT, deschide **Detalii** ale TOWER și adaugă depozitul primit. VAULT primește parola restic a TOWER prin grup; nimeni nu tastează o cheie.

**6. Fă-l reciproc.** Repetă pașii în sens invers: VAULT cere ca TOWER să-i fie receptor. Fiecare stație impune atunci append-only pentru cealaltă, și niciuna nu poate șterge copiile celeilalte.

Un rest-server pe care îl rulezi manual funcționează și el: adaugă-l cu **Adaugă destinație** ca rest-server, cu utilizatorul și parola din `.htpasswd`, și pornește protecția la ștergere. Păstrează `--append-only` în opțiunile lui; fără el partea îndepărtată este din nou o partajare obișnuită.

!!! warning "O cale locală este o cale din interiorul containerului"
    Acolo unde BombVault cere un folder de pe acest server, introdu-l relativ la montarea gazdei, de exemplu `user/appdata/rest-server/bombvault-containers`, nu `/mnt/user/appdata/...`. BombVault rulează într-un container, unde `/mnt` al gazdei este montat în alt loc; o cale absolută de gazdă nu există în interior. Dacă lipești una, BombVault îți spune calea relativă de folosit în loc.

## Asistent de urgență {#emergency}

**Asistentul de urgență** readuce un server pierdut sau reinstalat. Pornește pe pagina **Restaurare** la **Urgență** și de la **Restaurează dintr-o copie de rezervă** pe pagina de bun venit a unei instalări noi. Rulează în patru pași: **Acces**, **Setări**, **Selecție** și **Restaurare**.

1. **Acces.** Arată BombVault cheia vechilor copii, în unul din trei moduri:
    - **Încarcă kitul de recuperare**: fișierul descărcat în timpul configurării. Conține cheia și fiecare destinație, iar BombVault completează restul din el. Un kit sigilat cu age cere identitatea pentru care a fost sigilat; este folosită o singură dată și nu este stocată.
    - **Prin grup**: o instanță a grupului tău păstrează cheia ca depunere. Ai nevoie de cele douăsprezece cuvinte ale grupului, de codul tău de deblocare și de cineva la acea instanță. Vezi [Păstrătorul de cheie](#key-keeper).
    - **Introdu cheia**: `APP_KEY` al vechiului server și unde sunt copiile lui.

    BombVault încearcă apoi cheia pe fiecare destinație și spune pentru fiecare dacă este lizibilă, negăsită, blocată cu altă cheie, inaccesibilă sau încă fără autentificare. Încă nu s-a salvat nimic, iar cheia rămâne în memorie un timp limitat. **Preia cheia** face ca acest server să-și scrie de acum copiile cu acea cheie; **Anulează preluarea** revine la propria cheie până rulează prima copie. Un server care are deja copii proprii refuză preluarea și trimite la [Dintr-un folder vechi de copii](#restore-from-another-bombvault-repo). **Salvează credențialele** stochează autentificările pe care le poartă kitul pentru destinațiile la distanță și întreabă înainte să le înlocuiască pe cele pe care le are deja acest server.
2. **Setări.** **Readu setările** restaurează propriile setări ale BombVault din cea mai nouă copie Auto-backup: destinații, credențiale, programări și reguli. BombVault repornește ca să le aplice, prin socketul Docker, deci baza de date activă de setări nu este niciodată suprascrisă sub un handle deschis, iar asistentul continuă singur după aceea. Autentificarea acestui server rămâne: continui să te autentifici cu parola pe care o folosești acum. Secretele stocate de vechiul server care nu pot fi preluate sunt listate, ca să le poți introduce din nou.
3. **Selecție.** BombVault caută prin copiile tale și listează containerele, VM-urile, seturile de foldere și seturile de date ZFS pe care le găsește. Cea mai nouă copie de rezervă este preselectată peste tot. Ce are încă acest server pornește oprit, pentru că o restaurare l-ar înlocui. Seturile de foldere nu mai știu de unde au venit după o reconstruire, așa că merg într-un folder ales de tine.
4. **Restaurare.** Tot ce este selectat revine dintr-o dată, lăsat oprit dacă vrei să pornești tu fiecare container și VM, ceea ce este cel mai sigur după o pierdere totală. După aceea descarcă un kit de recuperare nou: cel vechi aparține instalării anterioare.

!!! tip "Migrare planificată versus dezastru"
    Asistentul de urgență restaurează propriile setări ale BombVault dintr-o copie. Pentru o mutare *planificată* pe o stație nouă poți în schimb să-ți muți configurația direct cu cardul **Exportă / importă setările** (un fișier JSON portabil). Vezi [Configurare](configuration.md#portable-settings-export-and-import).

### Dintr-un folder vechi de copii {#restore-from-another-bombvault-repo}

**Dintr-un folder vechi de copii**, la **Alte surse** pe pagina Restaurare, deschide depozitul unei instanțe BombVault *diferite* (o partajare montată sub `/mnt` sau o adresă la distanță) cu kitul de recuperare al acelei instanțe sau cu `APP_KEY` tastat, într-o sesiune unică, doar în citire. Răsfoiește containerele, VM-urile, seturile de foldere și seturile de date ZFS stocate acolo, alege o copie de rezervă și restaureaz-o, iar obiectul restaurat devine un container, VM sau set de foldere local obișnuit. Nimic nu se scrie vreodată în celălalt depozit, iar setările tale de copii rămân neatinse (sesiunea trăiește în memorie și expiră singură). Mutarea unui container de pe serverul A pe serverul B nu înseamnă să-ți reîndrepți setările și să le reîntorci după aceea. Această fereastră este una unică: deschide o sesiune, restaurează ce alegi și uită cealaltă instanță. Dacă vrei în schimb o rânduială permanentă, în care această stație preia după un program copiile de rezervă ale altei instanțe în propriul depozit, acesta este rolul **Preluator** de pe pagina Instanțe.

Un container a cărui rețea nu există pe acest server, de exemplu o rețea Unraid `br0` pe o gazdă Docker simplă, arată sub rândul lui un selector de rețea. BombVault o creează în rețeaua pe care o alegi, împreună cu celelalte rețele ale lui. IP-ul fix și adresa MAC aparțineau vechii rețele și sunt eliminate, așa că rețeaua nouă le atribuie.

## Păstrător de cheie {#key-keeper}

Un păstrător de cheie este o instanță a grupului tău care ține o copie sigilată a kitului de recuperare al acestui server. Nu poate deschide singur copia. Dacă acest server se pierde vreodată, o instalare nouă care se alătură grupului tău cere copia de la păstrător și o deschide cu codul tău de deblocare.

1. Pe pagina Instanțe, pornește **Păstrător de cheie** pe cardul instanței care ar trebui să o păstreze și alege **Fă depunerea**. Depunerea cere o parolă de conectare.
2. Acest server arată o singură dată **Codul de deblocare**: douăzeci de litere și cifre în patru grupuri de câte cinci. Notează-l și confirmă cu **Am notat codul**. Acest server nu stochează codul, ci doar ce îi trebuie ca să sigileze o depunere pentru el.
3. Păstrătorul ține acum depunerea, iar acest server o reînnoiește când se schimbă kitul. **Depunerea cheii** pe cardul acestei instanțe listează **Depunerea acestui server** cu fiecare păstrător și starea lui, și **Depunerile păstrate de acest server** pentru ceilalți. **Creează un cod nou** înlocuiește codul; fiecare păstrător trece la codul nou când primește noua depunere.

În acea zi ai nevoie de trei lucruri: cele douăsprezece cuvinte ale grupului tău, codul de deblocare și accesul la interfața păstrătorului, unde cineva trebuie să apese **Permite**. În Asistentul de urgență alege **Prin grup**, alege depunerea, cere-o și tastează codul de deblocare. Codul este folosit pe noul server și nu este trimis păstrătorului. Pe păstrător, cererea apare la **Depunerea cheii** cu cine o cere, dacă acea instanță a fost văzută vreodată acolo și de câte ori a cerut; **Permite** predă copia sigilată, iar permiterea cere o parolă de conectare. Instalarea nouă are 30 de minute ca s-o ridice după Permite, o cerere la care nimeni nu răspunde expiră după o zi, iar o instanță care cere prea des trebuie să aștepte o oră.

Fără cod nimeni nu poate deschide copia, nici păstrătorul. Dacă codul se pierde, creează unul nou cât timp acest server încă rulează; dacă serverul a dispărut deja, kitul de recuperare și `APP_KEY` tastat funcționează în continuare. Păstrătorul poate șterge o depunere oricând, iar proprietarul trebuie atunci să ceară din nou.

## Kit de recuperare {#recovery-kit}

Aceasta este piesa care face posibilă recuperarea după dezastru chiar și când nu rulează niciun BombVault.

**Descarcă kitul de recuperare** în cardul **Pentru urgențe** de la **Setări, Restaurare** îți dă **cheia**, **parola restic derivată**, fiecare **destinație** cu depozitul fiecărei secțiuni, autentificările de care are nevoie o instalare nouă ca să ajungă la ele și **comenzile restic exacte**, ca să poți restaura direct cu CLI-ul restic pe orice mașină. Cere o parolă de conectare. Kitul tipărește cheia și cu criptarea oprită, pentru că definițiile containerelor și ale VM-urilor sunt mereu sigilate cu ea. După o preluare tipărește cheia de rezervă preluată și `APP_KEY` al acestui server, de care o instalare nouă are nevoie doar pentru a readuce setările. Un card **Salvează-ți kitul de recuperare** din prezentare îți amintește până îl păstrezi.

!!! danger "Păstrează kitul de recuperare în afara serverului"
    Kitul conține secretul care decriptează copiile tale. Păstrează-l undeva în siguranță și separat de server (un manager de parole, o copie tipărită într-un seif). Dacă pierzi și BombVault, și `APP_KEY`, fără kit de recuperare și fără depunere de cheie, copiile tale criptate nu mai pot fi recuperate.

!!! warning "Cel mai nou instantaneu nu este mereu cel de restaurat"
    De la restic 0.17, `restic snapshots` arată dimensiunea fiecărui instantaneu. După o pierdere de date, cel mai nou instantaneu poate fi cel golit, așa că nu restaura un instantaneu mult mai mic decât cele dinaintea lui. După ransomware poate fi cel criptat, de dimensiunea obișnuită. Dacă BombVault încă rulează, verifică mai întâi pagina **Anomalii**: numește ultima copie bună, iar fereastra de restaurare o preselectează. O restaurare nu are nevoie de niciun dat de anomalii al BombVault, iar pauza de retenție doar păstrează mai multe instantanee.

### Sigilarea kitului

Dacă ai pornit **Criptează exporturile cu age** la **Setări, Sistem**, și kitul este sigilat cu el și se descarcă drept `bombvault-recovery-kit.md.age`. Este în armură ASCII, nu binar, deci rămâne text simplu: lipirea lui într-un manager de parole sau tipărirea funcționează exact ca înainte, conținutul pur și simplu nu poate fi citit fără cheia ta.

!!! warning "Nu păstra cheia age în interiorul kitului"
    Ai nevoie de cheia **privată** age ca să deschizi un kit sigilat. Păstreaz-o undeva care nu depinde de kitul însuși, altfel vei avea de recuperat două lucruri în loc de unul. Sigilarea merită când kitul este stocat undeva pe care nu-l controlezi pe deplin (un manager de parole partajat, notițe în cloud, o tipăritură într-un birou); un kit din propriul seif este deja protejat de seif.

    Cu criptarea pornită și fără niciun destinatar utilizabil configurat, descărcarea este refuzată categoric. BombVault nu revine niciodată la a da cheia principală în clar.

### Când kitul nu e la îndemână

Parola nu este stocată nicăieri, ea este **calculată** din cheia copiilor, așa că o poți reproduce singur cu nimic altceva decât cheia și un shell:

```sh
printf 'bombvault:restic-repo'   | openssl dgst -sha256 -mac HMAC -macopt hexkey:$APP_KEY -r   | cut -d' ' -f1
```

Aceasta este HMAC-SHA256 peste șirul fix `bombvault:restic-repo`, cu cheia ca octeții bruți ai `APP_KEY` hex, tipărit ca 64 de caractere hex cu litere mici. Aceeași valoare este în kit, listată ca parola restic derivată; asta este pentru ziua în care kitul e într-un loc unde nu ești. Pe un server care a preluat copiile altuia, folosește cheia de rezervă preluată, care este vechiul APP_KEY al acelui server.

!!! warning "Pentru un depozit primit, folosește cheia instanței CARE TRIMITE"
    Un depozit care a ajuns aici printr-o copie off-site a fost creat de mașina care l-a trimis, cu cheia **ei**. Derivarea din cheia stației care primește produce o parolă pe care restic o va respinge, ceea ce arată exact ca un depozit corupt și nu este. Acesta este motivul obișnuit pentru care `restic check` pe un depozit primit cere parola iar și iar.

Pentru că definițiile de recuperare se află **în interiorul** fiecărui depozit (`<repo>/def`, `<repo>/vm-def`), un folder de depozit copiat este complet autonom, deci kitul plus depozitul este tot ce are nevoie o restaurare pe hardware gol.

## Recuperarea unui dump de bază de date {#database-dumps}

Un dump de bază de date este un punct de restaurare propriu în depozitul containerelor, etichetat `dbdump:<container>`, care conține singurul fișier `/dbdump/<container>.sql`. BombVault le listează, le descarcă și le importă în fila **Copii de rezervă** a containerului; mai jos sunt aceiași pași doar cu restic, pentru ziua în care BombVault nu mai este.

```sh
restic -r <repo> snapshots --tag dbdump:<container>
restic -r <repo> dump --tag dbdump:<container> latest /dbdump/<container>.sql > <container>.sql
```

Etichetele `dbversion:` și `dbname:` de pe fiecare dump spun din ce versiune de server provine și ce baze de date conține. Un fișier complet se încheie cu `-- PostgreSQL database cluster dump complete` sau `-- Dump completed`.

Importă-l într-un container de aceeași versiune sau mai nouă (PostgreSQL), sau de aceeași versiune majoră (MySQL și MariaDB), pornit o dată cu un folder de date gol, ca să se inițializeze. Gazda nu are nevoie de niciun client de bază de date, containerul are unul:

```sh
docker exec -i <container> sh -c 'exec psql -X -U "${POSTGRES_USER:-postgres}" -d postgres' < <container>.sql
docker exec -i <container> sh -c 'exec mariadb -uroot -p"$MARIADB_ROOT_PASSWORD"' < <container>.sql
docker exec -i <container> sh -c 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < <container>.sql
```

Pentru o singură bază de date dintr-un dump complet, MySQL și MariaDB acceptă `--one-database <nume>` în comanda clientului. Un dump PostgreSQL are o secțiune pe bază de date, fiecare începând cu o linie `\connect <nume>`: copiază acea secțiune într-un fișier propriu și importă-o cu `-d <nume>` după ce ai creat baza de date.

!!! warning "Un dump făcut ca root poartă utilizatorii serverului"
    Un dump complet MySQL sau MariaDB făcut ca root conține baza de date de sistem `mysql`, deci importul lui înlocuiește conturile noului server, inclusiv parola root, cu cele din dump. Pe PostgreSQL, `role ... already exists` pentru utilizatorul creat de container este de așteptat și inofensiv.
